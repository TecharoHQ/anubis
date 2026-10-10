# Proxmox end to end testing

Prevents recurrence of [TecharoHQ/anubis#2015](https://github.com/TecharoHQ/anubis/discussions/2015). It spins up a new Ubuntu VM from a pool of potential candidates, tries to install Anubis on it, tries to start Anubis, and if it fails then this CI job fails and it creates a new VM for the pool.
