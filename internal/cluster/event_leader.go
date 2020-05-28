package cluster

import (
	"fmt"
	"hash_interface/internal/storage"
	"hash_interface/tools"
)

func (this *StateMachine) listenLeaderEventLoop() {

	heartbeatTimer := setHeartbeatTimer()

	for this.GetStatus() == Leader {
		select {

		case msg := <-*(this.ScheduleChannel):

			switch msg.Type {
			case AppendEntry:

				key := msg.Msg
				value := msg.Value

				tools.InfoLogger.Printf(
					"나는 Leader : AppendEntry 전달 받음 - (key, value) : (%s, %s)",
					key,
					value,
				)

				err := this.handleAppendEntry(key, value)

				(*msg.InterruptChannel) <- err
				break

			case VoteRequest:
				// 리더가 투표요청을 받은경우
				// 자신의 Heartbeat가 다른 노드에게 가고있지 않았다
				// = 자신의 네트워크가 Split되었다가 다시 합쳐졌다
				// or 다른 노드가 혼자 Split 되었다가 합쳐졌다.
				tools.InfoLogger.Printf(
					"나는 Leader : 투표 독려 요청 옴!, 내 term %d, 요청 term %d, 참가해야하나 ? %v",
					this.getTerm(),
					msg.Term,
					this.ShouldParticipate(msg.Term),
				)

				if !this.ShouldParticipate(msg.Term) {
					(*msg.InterruptChannel) <- fmt.Errorf(
						"이미 지난 Term입니다",
					)
					break
				}

				this.MetaDataLock.Lock()
				this.setTerm(msg.Term)
				this.becomeFollower()
				this.MetaDataLock.Unlock()

				(*msg.InterruptChannel) <- nil

			case UpdateEntry:
				// 팔로워로부터 업데이트 요청

				reqIndexStart := msg.IndexTime + 1

				leaderIdxTime := this.GetIndexTime(true)

				if reqIndexStart > leaderIdxTime {
					(*msg.InterruptChannel) <- fmt.Errorf(
						"리더의 Index가 더 전에 있습니다",
					)
					break
				}

				go this.updateFollowersWal(
					reqIndexStart,
					msg.From,
				)

				(*msg.InterruptChannel) <- nil

				break

			case Error:
				break

			case Heartbeat:
				// 자신이 Leader인데 Heartbeat를 받은 경우
				// 다른 리더가 생겨 난 것이다

				// 만약 다른 리더의 하트비트의 Term이 뒤쳐진 애라면
				// Split되어 리더가 되어 돌아온 친구이다

				this.MetaDataLock.Lock()
				curTerm := this.getTerm()
				this.MetaDataLock.Unlock()

				if curTerm > msg.Term {
					(*msg.InterruptChannel) <- fmt.Errorf(
						"이미 지난 Term입니다",
					)

					break
				}

				// 다른 리더의 하트비트의 Term이 앞서있다면
				// 자신이 Split되어 뒤쳐진 것일 수도 있다
				// 앞선 리더를 리더로 인정하고 자신은 Follwer가 된다

				// 서로 다른 리더간 데이터 동기화 로직은 TO-DO

				this.MetaDataLock.Lock()
				this.setTerm(msg.Term)
				this.setNewLeader(msg.NewLeader)
				this.becomeFollower()
				this.MetaDataLock.Unlock()

				// 데이터 동기화 진행

				(*msg.InterruptChannel) <- nil

			default:
				break
			}

		case <-heartbeatTimer:

			this.broadcastHeartbeat()
		}

		heartbeatTimer = setHeartbeatTimer()
	}
}

// 내부적으로 Write Lock
//
func (this *StateMachine) updateFollowersWal(
	startIdx uint64,
	follower string,
) {

	leaderIdxTime := this.GetIndexTime(true)

	this.WriteLock.Lock()

	idx := startIdx

	for idx <= leaderIdxTime {
		keyValuePair := this.WriteAheadLog[idx]

		err := this.sendWalUpdateMsg(
			follower,
			keyValuePair,
			idx,
		)

		if err != nil {
			tools.ErrorLogger.Printf(
				"팔로워의 Write Ahead Log 업데이트 중 에러! %s",
				err.Error(),
			)
			break
		}

		tools.InfoLogger.Printf(
			"팔로워 Write Ahead Log 업데이트 성공 - (key, value) : (%s, %s)",
			keyValuePair.Key,
			keyValuePair.Value,
		)

		idx += 1
	}

	this.WriteLock.Unlock()
}

// 내부적으로 Write Lock
//
func (this *StateMachine) handleAppendEntry(
	key, value string,
) error {
	// Queue에서 하나씩 꺼내서 전파
	this.WriteLock.Lock()

	// Write on self's WAL(write ahead log)
	nextIdx := this.GetIndexTime(true) + 1

	this.writeOnWal(
		nextIdx,
		key,
		value,
	)

	this.MetaDataLock.Lock()
	this.setIndexTime(this.IndexTime + 1)
	this.MetaDataLock.Unlock()

	// Tell Others to Write on Logs
	// 내부에서 대기
	err := this.broadcastAppendWal(
		key,
		value,
	)

	// 과반수 이상이 Log 작성 성공한 경우
	if err == nil {

		tools.InfoLogger.Println(
			"과반수 이상의 노드가 AppendWal 성공",
		)

		this.MetaDataLock.Lock()
		this.setCommitIndex(this.GetIndexTime(false))
		this.MetaDataLock.Unlock()

		this.broadcastCommit()

		// 자신의 Key Value Store 에 저장
		go saveData(storage.KeyValuePair{
			Key:   key,
			Value: value,
		})

	} else {
		tools.ErrorLogger.Printf(
			"과반수 이상의 노드가 AppendWal 실패 : %s",
			err.Error(),
		)
	}

	this.WriteLock.Unlock()

	return err
}
