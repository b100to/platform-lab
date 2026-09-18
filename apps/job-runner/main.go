package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type createJobRequest struct {
	Name string `json:"name"`
}

type Job struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

var jobs = []Job{
	{ID: "1", Name: "Job 1", Status: "running", CreatedAt: time.Now()},
	{ID: "2", Name: "Job 2", Status: "completed", CreatedAt: time.Now()},
}

func createJob(w http.ResponseWriter, r *http.Request) {
	var req createJobRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	job := Job{
		ID:        fmt.Sprintf("%d", len(jobs)+1),
		Name:      req.Name,
		Status:    "PENDING",
		CreatedAt: time.Now(),
	}

	jobs = append(jobs, job)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(job)
}

func healthz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "ok")
}

func jobHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func main() {
	http.HandleFunc("POST /api/jobs", createJob)
	http.HandleFunc("GET /healthz", healthz)
	http.HandleFunc("GET /api/jobs", jobHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
