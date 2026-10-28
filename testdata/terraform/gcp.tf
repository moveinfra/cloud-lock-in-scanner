resource "google_compute_network" "vpc" {
  name = "main"
}

resource "google_container_cluster" "primary" {
  name     = "primary"
  location = "europe-west1"
}

resource "google_bigquery_dataset" "events" {
  dataset_id = "events"
}

resource "google_pubsub_topic" "events" {
  name = "events"
}

resource "google_storage_bucket" "exports" {
  name     = "example-exports"
  location = "EU"
}
