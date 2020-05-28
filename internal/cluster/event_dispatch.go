package cluster

import "hash_interface/internal/storage"

type Dispatcher struct {
	ScheduleChannel *(chan ClusterMsg)
}

var EventDispatcher *Dispatcher

func (this *Dispatcher) Init(channel *(chan ClusterMsg)) {
	this.ScheduleChannel = channel
}

func (this *Dispatcher) DispatchHeartbeat(
	leader string,
	term, indexTime, commitIndex uint64,
	interruptChannel *(chan error),
) {
	*(this.ScheduleChannel) <- ClusterMsg{
		Type:             Heartbeat,
		NewLeader:        leader,
		Term:             term,
		CommitIndex:      commitIndex,
		IndexTime:        indexTime,
		InterruptChannel: interruptChannel,
	}
}

func (this *Dispatcher) DispatchTimeoutRefresh() {
	*(this.ScheduleChannel) <- ClusterMsg{
		Type: EmptyMsg,
	}
}

func (this *Dispatcher) DispatchPassToLeader(
	key, value string,
	interruptChannel *(chan error),
) {

	*(this.ScheduleChannel) <- ClusterMsg{
		Type:             ToLeader,
		Msg:              key,
		Value:            value,
		InterruptChannel: interruptChannel,
	}

}

func (this *Dispatcher) DispatchAppendWal(
	keyValuPair storage.KeyValuePair,
	metaDataMap map[string]interface{},
	interruptChannel *(chan error),
) {

	*(this.ScheduleChannel) <- ClusterMsg{
		Type:             AppendEntry,
		Msg:              keyValuPair.Key,
		Value:            keyValuPair.Value,
		IndexTime:        metaDataMap[IndexTimeHeader].(uint64),
		MetaData:         metaDataMap,
		InterruptChannel: interruptChannel,
	}

}

func (this *Dispatcher) DispatchStartUpdateFollowerWal(
	follower string,
	followerIdxTime uint64,
	interruptChannel *(chan error),
) {

	*(this.ScheduleChannel) <- ClusterMsg{
		Type:             UpdateEntry,
		From:             follower,
		IndexTime:        followerIdxTime,
		InterruptChannel: interruptChannel,
	}
}

func (this *Dispatcher) DispatchUpdateWalFromLeaader(
	keyValuPair storage.KeyValuePair,
	targetIdx uint64,
	interruptChannel *(chan error),
) {

	*(this.ScheduleChannel) <- ClusterMsg{
		Type:             UpdateEntry,
		Msg:              keyValuPair.Key,
		Value:            keyValuPair.Value,
		IndexTime:        targetIdx,
		InterruptChannel: interruptChannel,
	}

}

func (this *Dispatcher) DispatchCommit(
	commitIdx, term uint64,
	interruptChannel *(chan error),
) {

	*(this.ScheduleChannel) <- ClusterMsg{
		Type:             Commit,
		CommitIndex:      commitIdx,
		Term:             term,
		InterruptChannel: interruptChannel,
	}

}

func (this *Dispatcher) DispatchVote(
	term uint64,
	interruptChannel *(chan error),

) {

	*(this.ScheduleChannel) <- ClusterMsg{
		Type:             VoteRequest,
		Term:             term,
		InterruptChannel: interruptChannel,
	}
}
