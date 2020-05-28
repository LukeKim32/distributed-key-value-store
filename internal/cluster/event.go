package cluster

import (
	"hash_interface/internal/storage"
	"hash_interface/tools"
	"math/rand"
	"time"
)

const (
	// Unix nanosecond 기준
	MinTimeout = 150 * time.Millisecond // 1 * time.Second //
	MaxTimeout = 300 * time.Millisecond // 2 * time.Second //
)

func (this *StateMachine) StartEventLoop() {

	this.becomeFollower()

	for this.GetStatus() != Stopped {

		status := this.GetStatus()

		tools.InfoLogger.Printf(
			"%s 이벤트 루프 시작!",
			status,
		)

		switch status {
		case Follower:
			this.listenFollowerEventLoop()

		case Leader:
			this.listenLeaderEventLoop()

		case Candidate:
			this.listenCandidateEventLoop()

		}
	}
}

func (this *StateMachine) writeOnWal(idx uint64, key, value string) {

	if idx >= uint64(len(this.WriteAheadLog)) {
		newWriteAheadLog := make(
			[]storage.KeyValuePair,
			idx*2,
		)

		for i, keyValuePair := range this.WriteAheadLog {
			newWriteAheadLog[i] = keyValuePair
		}
	}

	this.WriteAheadLog[idx].Key = key
	this.WriteAheadLog[idx].Value = value
}

func setMsgTimeOut() <-chan time.Time {

	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	randDuration, diff := MinTimeout, (MaxTimeout - MinTimeout)
	if diff > 0 {
		randDuration += time.Duration(rand.Int63n(int64(diff)))
	}

	return time.After(randDuration)
}

func setHeartbeatTimer() <-chan time.Time {

	return time.After(50 * time.Millisecond)
}
