# Raft (consensus algorithm)

This is an implemetation of the `raft` algorithm in Golang. The skeleton of the project comes from MIT 6.824.

These are useful resources to understand `raft`:

- [Extended Raft paper](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf)
- [MIT 6.824](https://pdos.csail.mit.edu/6.824/schedule.html)   (see lectures 5, 6, 7)
- [Student guide to raft](https://thesquareplanet.com/blog/students-guide-to-raft/)

raft/raft.go contains all of the algorithm. Other files are used for testing purposes. To test raft.go `cd` to raft directory  and type the command `go test -run 2A`, `2B` or `2C`. For more information see this [link](https://pdos.csail.mit.edu/6.824/labs/lab-raft.html).
