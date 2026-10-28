resource "digitalocean_droplet" "web" {
  image  = "ubuntu-24-04-x64"
  size   = "s-2vcpu-4gb"
  region = "fra1"
}

resource "digitalocean_database_cluster" "cache" {
  name   = "cache"
  engine = "valkey"
  size   = "db-s-1vcpu-1gb"
}

resource "digitalocean_database_cluster" "pg" {
  name   = "pg"
  engine = "pg"
}
