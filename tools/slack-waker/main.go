// Command slack-waker turns a Slack slash command into a WakeRequest.
//
// The bot is deliberately thin. It does not decide who may wake what, how long
// a namespace may stay up, or when to put it back to sleep — the API server
// and the idle-reaper controller own all three. What it does is translate a
// sentence typed in a channel into an object in the right namespace, and
// report back what the cluster said.
//
// That split matters for more than tidiness: a bot that held the policy would
// be the thing an attacker needs to compromise, and a bot that held the timer
// would forget everything it was tracking when it restarted.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var wakeRequestGVR = schema.GroupVersionResource{
	Group:    "finops.b100to.dev",
	Version:  "v1alpha1",
	Resource: "wakerequests",
}

// durationPattern matches the leading duration of a command, e.g. "3h", "90m".
var durationPattern = regexp.MustCompile(`^([0-9]+(\.[0-9]+)?(s|m|h))+$`)

func main() {
	appToken := mustEnv("SLACK_APP_TOKEN")
	botToken := mustEnv("SLACK_BOT_TOKEN")

	// Which channel may act on which namespace is configuration, not
	// something the bot infers from a naming convention: a convention would
	// silently grant a new channel access to a namespace the moment someone
	// named it conveniently.
	channels, err := parseChannelMap(mustEnv("CHANNEL_NAMESPACES"))
	if err != nil {
		log.Fatalf("CHANNEL_NAMESPACES: %v", err)
	}

	k8s, err := newDynamicClient()
	if err != nil {
		log.Fatalf("kubernetes client: %v", err)
	}

	api := slack.New(botToken, slack.OptionAppLevelToken(appToken))
	client := socketmode.New(api)

	go handleEvents(client, api, k8s, channels)

	log.Printf("connecting over socket mode, serving %d channel(s)", len(channels))
	if err := client.Run(); err != nil {
		log.Fatalf("socket mode: %v", err)
	}
}

func handleEvents(client *socketmode.Client, api *slack.Client, k8s dynamic.Interface, channels map[string]string) {
	for evt := range client.Events {
		if evt.Type != socketmode.EventTypeSlashCommand {
			continue
		}
		cmd, ok := evt.Data.(slack.SlashCommand)
		if !ok {
			continue
		}
		// Acknowledge first. Slack gives three seconds before it shows the
		// user an error, which is not long enough to be sure an API call has
		// returned.
		client.Ack(*evt.Request)

		msg := handleWake(context.Background(), api, k8s, channels, cmd)
		if _, _, err := api.PostMessage(cmd.ChannelID,
			slack.MsgOptionText(msg, false),
			slack.MsgOptionResponseURL(cmd.ResponseURL, slack.ResponseTypeEphemeral),
		); err != nil {
			log.Printf("reply failed: %v", err)
		}
	}
}

// handleWake turns one command into one WakeRequest, and returns what to say
// back.
func handleWake(
	ctx context.Context,
	api *slack.Client,
	k8s dynamic.Interface,
	channels map[string]string,
	cmd slack.SlashCommand,
) string {
	namespace, ok := channels[cmd.ChannelName]
	if !ok {
		return fmt.Sprintf(
			":no_entry: `#%s` is not mapped to a namespace. Run this in the channel for the environment you mean.",
			cmd.ChannelName)
	}

	duration, reason, err := parseCommand(cmd.Text)
	if err != nil {
		return fmt.Sprintf(":warning: %v\nUsage: `/wake 3h deploying a hotfix`", err)
	}

	requestedBy := describeUser(ctx, api, cmd)

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "finops.b100to.dev/v1alpha1",
		"kind":       "WakeRequest",
		"metadata": map[string]any{
			"generateName": "wake-",
			"namespace":    namespace,
			"labels": map[string]any{
				"finops.b100to.dev/source":  "slack",
				"finops.b100to.dev/channel": cmd.ChannelName,
			},
		},
		"spec": map[string]any{
			"duration":    duration,
			"reason":      reason,
			"requestedBy": requestedBy,
		},
	}}

	created, err := k8s.Resource(wakeRequestGVR).Namespace(namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return describeCreateError(namespace, err)
	}

	return fmt.Sprintf(
		":white_check_mark: `%s` will stay awake for *%s*.\nRequest `%s` — it expires on its own, nothing to undo.\nReason: _%s_",
		namespace, duration, created.GetName(), reason)
}

// parseCommand splits "3h deploying a hotfix" into its duration and reason.
func parseCommand(text string) (duration, reason string, err error) {
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return "", "", errors.New("a duration and a reason are both required")
	}

	duration = fields[0]
	if !durationPattern.MatchString(duration) {
		return "", "", fmt.Errorf("%q is not a duration — try 30m, 3h, 90m", duration)
	}

	reason = strings.TrimSpace(strings.Join(fields[1:], " "))
	if len(reason) < 3 {
		return "", "", errors.New("the reason is too short to be useful later")
	}
	return duration, reason, nil
}

// describeUser resolves who typed the command.
//
// This is a label for people reading the object later, not an authorization
// input. Anything the bot puts in the spec is only as trustworthy as the bot,
// so the record that counts is the API server's own audit log.
func describeUser(ctx context.Context, api *slack.Client, cmd slack.SlashCommand) string {
	user, err := api.GetUserInfoContext(ctx, cmd.UserID)
	if err != nil {
		return cmd.UserName
	}
	if user.Profile.Email != "" {
		return user.Profile.Email
	}
	return user.Name
}

// describeCreateError turns an API rejection into something a developer can
// act on without reading Kubernetes errors.
func describeCreateError(namespace string, err error) string {
	switch {
	case apierrors.IsForbidden(err):
		return fmt.Sprintf(":no_entry: not allowed to create wake requests in `%s`.", namespace)
	case apierrors.IsInvalid(err):
		return fmt.Sprintf(":warning: the cluster rejected that request: %v", err)
	case apierrors.IsNotFound(err):
		return fmt.Sprintf(":question: `%s` has no WakeRequest API — is idle-reaper installed?", namespace)
	default:
		log.Printf("create failed in %s: %v", namespace, err)
		return ":x: could not reach the cluster. The error is in the bot's logs."
	}
}

// parseChannelMap reads "team-a-dev=team-a,team-b-dev=team-b".
func parseChannelMap(raw string) (map[string]string, error) {
	out := map[string]string{}
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		channel, namespace, found := strings.Cut(pair, "=")
		if !found || channel == "" || namespace == "" {
			return nil, fmt.Errorf("expected channel=namespace, got %q", pair)
		}
		out[strings.TrimPrefix(strings.TrimSpace(channel), "#")] = strings.TrimSpace(namespace)
	}
	if len(out) == 0 {
		return nil, errors.New("no channel mappings configured")
	}
	return out, nil
}

func newDynamicClient() (dynamic.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		// Outside the cluster, fall back to the local kubeconfig so the bot
		// can be run against kind without being deployed first.
		config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		).ClientConfig()
		if err != nil {
			return nil, err
		}
	}
	config.Timeout = 10 * time.Second
	return dynamic.NewForConfig(config)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s is required", key)
	}
	return v
}
