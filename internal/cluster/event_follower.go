package cluster

import (
	"fmt"
	"hash_interface/tools"
)

func (this *StateMachine) listenFollowerEventLoop() {

	timeoutChannel := setMsgTimeOut()
	this.SetIsUpdateingFromLeader(false)

	for this.GetStatus() == Follower {
		select {

		case msg := <-*(this.ScheduleChannel):

			switch msg.Type {

			case AppendEntry:

				leaderTerm, isSet := msg.MetaData[TermHeader]
				if !isSet {
					(*msg.InterruptChannel) <- fmt.Errorf(
						"Cluster 통신 - Term이 set되지 않았습니다",
					)
					break
				}

				this.MetaDataLock.Lock()
				term := this.getTerm()
				indexTime := this.GetIndexTime(false)
				this.MetaDataLock.Unlock()

				tools.InfoLogger.Printf(
					"리더로부터 받은 Append Entry - (key, value) : (%s, %s)",
					msg.Msg,
					msg.Value,
				)

				if term > leaderTerm.(uint64) {

					tools.ErrorLogger.Printf(
						"Append Entry 에러 : 리더의 Term이 더 이전입니다",
					)

					*msg.InterruptChannel <- fmt.Errorf(
						"Follower의 Term이 더 큽니다",
					)
					break
				}

				leader, isSet := msg.MetaData[LeaderHeader]
				if !isSet {

					tools.ErrorLogger.Println(
						"Cluster 통신 - 헤더에 Leader이 set되지 않았습니다",
					)

					(*msg.InterruptChannel) <- fmt.Errorf(
						"Cluster 통신 - Leader이 set되지 않았습니다",
					)
					break
				}

				this.MetaDataLock.Lock()
				this.setNewLeader(leader.(string))
				this.MetaDataLock.Unlock()

				if term == leaderTerm {

					leaderIndexTime, isSet := msg.MetaData[IndexTimeHeader]
					if !isSet {
						(*msg.InterruptChannel) <- fmt.Errorf(
							"Cluster 통신 - Index Time이 set되지 않았습니다",
						)
					}

					indexTimeDiff := leaderIndexTime.(uint64) - indexTime

					tools.InfoLogger.Printf(
						"리더 AppendEntry의 Index Time 차이 : %d",
						indexTimeDiff,
					)

					if indexTimeDiff == 1 {

						msg.IndexTime = leaderIndexTime.(uint64)

						go this.updateEntryFromLeader(msg)

						break

					} else if indexTimeDiff > 1 {
						// 현재 노드가 up-to-date가 아니다
						// 동일한 Term인데, AppendEntry 패킷이 손실되어 못받은 경우

						// HTTP Request 쓰레드의 대기를 우선 종료
						(*msg.InterruptChannel) <- fmt.Errorf(
							"데이터가 not up-to-date",
						)

						tools.InfoLogger.Printf(
							"내 Index가 뒤쳐져 있어서 리더에게 업데이트 요청",
						)

						this.MetaDataLock.Lock()
						leader := this.GetLeader()
						this.MetaDataLock.Unlock()

						if leader == "" {
							break
						}

						this.UpdateFromLeaderLock.Lock()

						isUpdatingFromLeader := this.IsUpdateingFromLeader()

						if isUpdatingFromLeader {
							break
						}

						this.SetIsUpdateingFromLeader(true)

						this.UpdateFromLeaderLock.Unlock()

						// 밀린 데이터 요청
						go AskLeaderForUpdate(
							leader,
							this.Cluster.curIpAddress,
							indexTime,
							this.ScheduleChannel,
						)

					} else {
						// Follower인 내 Term 이 리더로부터 받은 AE의 Term 보다 앞서있는 경우
						// 이는, SPlit 되어 리더가 두명이었다가 다시 합쳐진 뒤,
						// split 되었을 때 반대쪽 리더로부터 AE를 받은 경우다

						tools.ErrorLogger.Printf(
							"Append Entry 에러 : 리더의 AppendEntry의 Index Time이 더 이전입니다",
						)

						// 데이터 동기화 필요
						(*msg.InterruptChannel) <- fmt.Errorf(
							"리더의 Index Time보다 앞서나갈 수 없습니다",
						)
					}

				} else if this.Term < leaderTerm.(uint64) {
					// Term이 뒤쳐진 경우

					tools.InfoLogger.Printf(
						"내 Term이 리더의 Term 보다 뒤쳐져 있습니다",
					)

					// HTTP Request 쓰레드의 대기를 우선 종료
					(*msg.InterruptChannel) <- fmt.Errorf(
						"데이터가 not up-to-date",
					)

					// Test //
					this.MetaDataLock.Lock()
					leader := this.GetLeader()
					this.MetaDataLock.Unlock()

					if leader == "" {
						break
					}

					this.UpdateFromLeaderLock.Lock()

					isUpdatingFromLeader := this.IsUpdateingFromLeader()
					if isUpdatingFromLeader {
						break
					}

					this.SetIsUpdateingFromLeader(true)

					this.UpdateFromLeaderLock.Unlock()

					// 밀린 데이터 요청
					go AskLeaderForUpdate(
						leader,
						this.Cluster.curIpAddress,
						indexTime,
						this.ScheduleChannel,
					)

				} else {

					tools.ErrorLogger.Printf(
						"Append Entry 오류 : 리더의 Term이 더 이전입니다",
					)

					(*msg.InterruptChannel) <- fmt.Errorf(
						"리더의 Term보다 앞서나갈 수 없습니다",
					)
					break
				}

				// 이전 Term은 무시

				(*msg.InterruptChannel) <- nil

			case ToLeader:

				err := this.SendToLeader(msg.Msg, msg.Value)
				(*msg.InterruptChannel) <- err

			case VoteRequest:

				tools.InfoLogger.Printf(
					"나는 Follower : 투표 독려 요청 옴, 내 term %d, 요청 term %d, 참가해야하나 ? %v",
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
				this.MetaDataLock.Unlock()

				(*msg.InterruptChannel) <- nil

			case UpdateFinished:
				// 리더로부터 업데이트 완료 신호 받았을 때

				this.SetIsUpdateingFromLeader(false)

			case UpdateEntry:

				go this.updateEntryFromLeader(msg)

			case Commit:

				go this.handleCommitMsg(msg)

			case Error:
				break

			case Heartbeat:

				this.handleLeaderHeartbeat(
					msg.Term,
					msg.IndexTime,
					msg.CommitIndex,
					msg.NewLeader,
				)

				*(msg.InterruptChannel) <- nil

			default:
				// HeartBeat로 온 Index와 자신의 인덱스의 차이가 1보다 클 경우,
				// update request

			}

		case <-timeoutChannel:
			this.becomeCandidate()
		}

		timeoutChannel = setMsgTimeOut()
	}
}

// Interrupt Message도 전송
//
func (this *StateMachine) updateEntryFromLeader(msg ClusterMsg) {

	this.WriteLock.Lock()

	targetIdx := msg.IndexTime
	key := msg.Msg
	value := msg.Value

	this.writeOnWal(
		targetIdx,
		key,
		value,
	)

	this.MetaDataLock.Lock()

	curIdxTime := this.GetIndexTime(false)

	if curIdxTime < targetIdx {

		tools.InfoLogger.Printf(
			"인덱스 갱신 %d => %d",
			curIdxTime,
			targetIdx,
		)

		this.setIndexTime(targetIdx)
	}

	this.MetaDataLock.Unlock()

	this.WriteLock.Unlock()

	tools.InfoLogger.Printf(
		"Write Ahead Log에 추가 성공 : 타겟인덱스 : %d / (키, 밸류) : (%s, %s)",
		targetIdx,
		key,
		value,
	)

	*(msg.InterruptChannel) <- nil
}

// 고루틴으로 돌아간다
//
func (this *StateMachine) handleCommitMsg(msg ClusterMsg) {

	this.MetaDataLock.Lock()
	curCommitIdx := this.getCommitIdx()
	curTerm := this.getTerm()
	this.MetaDataLock.Unlock()

	newCommitIdx := msg.CommitIndex

	tools.InfoLogger.Printf(
		"리더로부터 커밋 메세지 도착 : 새로운 인덱스 %d (기존 인덱스 %d)",
		newCommitIdx,
		curCommitIdx,
	)

	if msg.Term < curTerm {

		tools.ErrorLogger.Println(
			"올바르지 않은 리더의 Term 입니다.",
		)

		*(msg.InterruptChannel) <- fmt.Errorf(
			"올바르지 않은 리더의 Term 입니다.",
		)
		return
	}

	this.MetaDataLock.Lock()
	leader := this.GetLeader()
	this.MetaDataLock.Unlock()

	if leader == "" {
		tools.ErrorLogger.Println(
			"등록된 리더가 없습니다",
		)
		*(msg.InterruptChannel) <- nil
		return
	}

	if curCommitIdx >= newCommitIdx {
		tools.ErrorLogger.Println(
			"리더의 커밋인덱스가 더 작습니다",
		)
		*(msg.InterruptChannel) <- nil
		return
	}

	walLength := uint64(len(this.WriteAheadLog))

	// 요청온 Commit Index가 내 WAL 범위 밖, 즉 AppendEntry Update 데이터를 아직 못받았는데
	// Commit이 그냥 날아온 경우
	if walLength <= newCommitIdx {

		// HTTP Request 쓰레드의 대기를 우선 종료
		*(msg.InterruptChannel) <- fmt.Errorf(
			"Write Ahead Log가 최신이 아닙니다",
		)

		tools.ErrorLogger.Println(
			"Write Ahead Log가 최신이 아닙니다",
		)

		curIdx := this.GetIndexTime(true)

		// 업데이트 중복 요청 방지
		this.UpdateFromLeaderLock.Lock()

		isUpdatingFromLeader := this.IsUpdateingFromLeader()

		if isUpdatingFromLeader {
			return
		}

		this.SetIsUpdateingFromLeader(true)

		this.UpdateFromLeaderLock.Unlock()

		// 밀린 데이터 요청
		AskLeaderForUpdate(
			leader,
			this.Cluster.curIpAddress,
			curIdx,
			this.ScheduleChannel,
		)

		return
	}

	tools.InfoLogger.Println(
		"Write Ahead Log 데이터 커밋 시작",
	)

	this.UpdateFromLeaderLock.Lock()

	isUpdatingFromLeader := this.IsUpdateingFromLeader()

	if isUpdatingFromLeader {
		*(msg.InterruptChannel) <- nil
		return
	}

	this.SetIsUpdateingFromLeader(true)

	this.UpdateFromLeaderLock.Unlock()

	// write lock 이 걸려있다!
	startIdxToPurge, err := this.saveCommitedData(newCommitIdx)

	if err != nil {

		*(msg.InterruptChannel) <- fmt.Errorf(
			"Write Ahead Log의 오류가 존재합니다",
		)

		tools.ErrorLogger.Printf(
			"Write Ahead Log의 %d번째 인덱스에 오류가 발생했습니다",
			startIdxToPurge,
		)

		this.UpdateFromLeaderLock.Lock()

		isUpdatingFromLeader := this.IsUpdateingFromLeader()
		if isUpdatingFromLeader {
			return
		}

		this.SetIsUpdateingFromLeader(true)

		this.UpdateFromLeaderLock.Unlock()

		// 밀린 데이터 요청
		AskLeaderForUpdate(
			leader,
			this.Cluster.curIpAddress,
			startIdxToPurge,
			this.ScheduleChannel,
		)

		return
	}

	// 커밋이 문제없이 완료되었다

	// -------------디버깅용 로그-------------
	if curCommitIdx < newCommitIdx {

		tools.InfoLogger.Printf(
			"커밋 성공! (커밋 인덱스 %d) ",
			newCommitIdx,
		)

	} else if curCommitIdx == newCommitIdx {

		tools.InfoLogger.Printf(
			"커밋이 이미 되어 있습니다 (커밋 인덱스 %d) ",
			newCommitIdx,
		)
	}
	// -------------디버깅용 로그-------------

	*(msg.InterruptChannel) <- nil

}

// NOT A GOROUTINE CALL
//
func (this *StateMachine) handleLeaderHeartbeat(
	leaderTerm, leaderIdxTime, leaderCommitIdx uint64,
	leader string,
) {

	this.MetaDataLock.Lock()
	curTerm := this.getTerm()
	commitIdx := this.getCommitIdx()
	curIdxTime := this.GetIndexTime(false)
	this.MetaDataLock.Unlock()

	if leaderTerm >= curTerm {

		this.MetaDataLock.Lock()
		this.setTerm(leaderTerm)
		this.setNewLeader(leader)
		this.MetaDataLock.Unlock()

		// 인덱스가 밀려있는 경우
		if leaderIdxTime > curIdxTime {

			this.UpdateFromLeaderLock.Lock()

			isUpdatingFromLeader := this.IsUpdateingFromLeader()

			if isUpdatingFromLeader {
				return
			}

			this.SetIsUpdateingFromLeader(true)

			this.UpdateFromLeaderLock.Unlock()

			curIdxTime = this.GetIndexTime(true)

			if leaderIdxTime > curIdxTime {
				// 데이터를 요청한다
				go AskLeaderForUpdate(
					leader,
					this.Cluster.curIpAddress,
					curIdxTime,
					this.ScheduleChannel,
				)
			}

		} else if leaderIdxTime == curIdxTime {

			// 커밋이 밀려있는 경우
			if commitIdx < leaderCommitIdx {

				this.UpdateFromLeaderLock.Lock()

				isUpdatingFromLeader := this.IsUpdateingFromLeader()
				if isUpdatingFromLeader {
					return
				}

				this.SetIsUpdateingFromLeader(true)

				this.UpdateFromLeaderLock.Unlock()

				this.MetaDataLock.Lock()
				commitIdx = this.getCommitIdx()
				this.MetaDataLock.Unlock()

				if commitIdx >= leaderCommitIdx {
					return
				}

				// 커밋을 위해 별도의 고루틴에 맡긴다
				go func() {

					startIdxToPurge, err := this.saveCommitedData(
						leaderCommitIdx,
					)

					if err != nil {

						tools.ErrorLogger.Printf(
							"Write Ahead Log의 %d번째 인덱스에 오류가 발생했습니다",
							startIdxToPurge,
						)

						this.UpdateFromLeaderLock.Lock()

						isUpdatingFromLeader := this.IsUpdateingFromLeader()

						if isUpdatingFromLeader {
							return
						}

						this.SetIsUpdateingFromLeader(true)

						this.UpdateFromLeaderLock.Unlock()

						// 밀린 데이터 요청
						AskLeaderForUpdate(
							leader,
							this.Cluster.curIpAddress,
							startIdxToPurge,
							this.ScheduleChannel,
						)

						return
					}
				}()

			}

		}

	}
}

// 내부적으로 WriteLock 을 걸어준다!
//
func (this *StateMachine) saveCommitedData(
	leaderCommitIdx uint64,
) (uint64, error) {

	this.MetaDataLock.Lock()
	commitIdx := this.getCommitIdx()
	this.MetaDataLock.Unlock()

	this.WriteLock.Lock()

	idx := commitIdx + 1
	isWalContaminated := false
	startIdxToPurge := idx

	tools.InfoLogger.Printf(
		"커밋 전, 기존 커밋 인덱스 %d => 목표 커밋 인덱스 %d",
		commitIdx,
		leaderCommitIdx,
	)

	for idx <= leaderCommitIdx {

		keyValuePair := this.WriteAheadLog[idx]

		if keyValuePair.Key == "" {
			isWalContaminated = true
			startIdxToPurge = idx
			break
		}

		err := saveData(
			this.WriteAheadLog[idx],
		)

		if err != nil {
			tools.ErrorLogger.Printf(
				"Key Value Store에 커밋 에러 발생 (인덱스 : %d) - %s",
				idx,
				err.Error(),
			)
			break
		}

		this.MetaDataLock.Lock()
		this.setCommitIndex(idx)
		this.MetaDataLock.Unlock()

		tools.InfoLogger.Printf(
			"커밋 인덱스 갱신 %d => %d",
			idx-1,
			idx,
		)

		idx += 1
	}

	this.WriteLock.Unlock()

	*this.ScheduleChannel <- ClusterMsg{
		Type: UpdateFinished,
	}

	if isWalContaminated {
		return startIdxToPurge, fmt.Errorf("Wal이 오염되었습니다")
	}

	return 0, nil
}
