package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alabarjasteh/raft-implementation/labrpc"
)

// import "bytes"
// import "github.com/alabarjasteh/raft-implementation/labgob"

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

type State string

const (
	Follower  State = "follower"
	Candidate       = "candidate"
	Leader          = "leader"
)

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	state State

	currentTerm int
	votedFor    int
	lastTime    time.Time // Reset each time get a heartbeat from current leader or grant a vote
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here (2A).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	term = rf.currentTerm
	isleader = (rf.state == Leader)
	return term, isleader
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

type AppendEntriesArgs struct {
	Term     int
	LeaderId int
}

type AppendEntriesReply struct {
	Term    int
	Success bool
}

func (rf *Raft) HandleAppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	DPrintf("[%d] Received AppendEntries from %d in term %d", rf.me, args.LeaderId, args.Term)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.currentTerm < args.Term {
		rf.stepDown(args.Term)
	}
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}
	rf.state = Follower // in case it is candidate
	rf.lastTime = time.Now()
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.HandleAppendEntries", args, reply)
	return ok
}

func (rf *Raft) callAppendEntries(server int, term int) bool {
	args := AppendEntriesArgs{
		Term:     term,
		LeaderId: rf.me,
	}
	var reply AppendEntriesReply

	ok := rf.sendAppendEntries(server, &args, &reply)
	if !ok {
		return false
	}

	rf.mu.Lock()
	if rf.currentTerm < reply.Term {
		rf.stepDown(reply.Term)
	}
	rf.mu.Unlock()

	return true // 2B
}

func (rf *Raft) priodicHeartbeat() {
	for {
		DPrintf("[%d] heartbeating", rf.me)
		rf.mu.Lock()
		if rf.state != Leader {
			rf.mu.Unlock()
			return
		}
		term := rf.currentTerm
		rf.lastTime = time.Now()
		rf.mu.Unlock()

		for i := range rf.peers {
			if i == rf.me {
				continue
			}
			go func(server int) {
				rf.callAppendEntries(server, term)
			}(i)
		}
		time.Sleep(150 * time.Millisecond)
	}
}

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term        int
	CandidateId int
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int
	VoteGranted bool
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.HandleRequestVote", args, reply)
	return ok
}

func (rf *Raft) HandleRequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	DPrintf("[%d] received request vote from %d for term %d", rf.me, args.CandidateId, args.Term)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.currentTerm < args.Term {
		rf.stepDown(args.Term)
	}

	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}
	if rf.votedFor == -1 || rf.votedFor == args.CandidateId {
		//TODO:	Check if candidate log is as up-to-date as receiver log (2B)
		reply.Term = rf.currentTerm
		reply.VoteGranted = true
		rf.votedFor = args.CandidateId
		rf.lastTime = time.Now()
	}
}

//No lock here, this funcion must be used when hold the lock
func (rf *Raft) stepDown(higherTerm int) {
	DPrintf("[%d] stepped down", rf.me)
	rf.currentTerm = higherTerm
	rf.state = Follower
	rf.votedFor = -1
}

func (rf *Raft) callRequestVote(server int, term int) bool {
	DPrintf("[%d] sending request vote to %d", rf.me, server)
	args := RequestVoteArgs{
		Term:        term,
		CandidateId: rf.me,
	}
	var reply RequestVoteReply
	ok := rf.sendRequestVote(server, &args, &reply)
	DPrintf("[%d] finish sending request vote to %d", rf.me, server)
	if !ok {
		return false
	}

	rf.mu.Lock()
	if rf.currentTerm < reply.Term {
		rf.stepDown(reply.Term)
	}
	rf.mu.Unlock()

	return reply.VoteGranted
}

func (rf *Raft) attemptElection() {
	rf.mu.Lock()
	rf.state = Candidate
	rf.currentTerm++
	rf.votedFor = rf.me
	rf.lastTime = time.Now()
	DPrintf("[%d] attempting an election at term %d", rf.me, rf.currentTerm)
	term := rf.currentTerm
	length := len(rf.peers)
	rf.mu.Unlock()

	var mu sync.Mutex
	cond := sync.NewCond(&mu)
	votes := 1
	finished := 1

	for i := 0; i < length; i++ {
		if i == rf.me {
			continue
		}
		go func(server int) {
			voteGranted := rf.callRequestVote(server, term)
			mu.Lock()
			defer mu.Unlock()
			if voteGranted {
				votes++
				DPrintf("[%d] got vote from  %d for term %d", rf.me, server, term)
			}
			finished++
			cond.Broadcast()
		}(i)
	}

	mu.Lock()
	if votes <= length/2 && finished != length {
		cond.Wait()
	}
	if votes > length/2 {
		DPrintf("[%d] we got enough votes, we are the leader for term %d", rf.me, rf.currentTerm)
		if rf.state != Candidate || rf.currentTerm != term {
			DPrintf("[%d] Term or state changed in the middle of election(current state: %v)", rf.me, rf.state)
			mu.Unlock()
			return
		}
		rf.state = Leader
		rf.priodicHeartbeat()
	} else {
		// lost the election
		DPrintf("[%d] lost the election", rf.me)
	}
	mu.Unlock()
}

func (rf *Raft) electionTask() {
	timeout := 300 + rand.Intn(900) //Each raft server has a random timeout but it is fixed for that raft
	DPrintf("[%d] timeout = %d ms", rf.me, timeout)
	for {
		rf.mu.Lock()
		interval := int(time.Now().Sub(rf.lastTime).Milliseconds())
		if interval >= timeout {
			go rf.attemptElection()
		}
		rf.mu.Unlock()
		time.Sleep(30 * time.Millisecond)
	}
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).

	return index, term, isLeader

}

//
// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
//
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	rf.state = Follower
	rf.currentTerm = 0
	rf.votedFor = -1

	// Your initialization code here (2A, 2B, 2C).
	go rf.electionTask()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	return rf
}
