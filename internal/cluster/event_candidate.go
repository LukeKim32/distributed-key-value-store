package cluster

import (
	"fmt"
	"hash_interface/tools"
	"time"
)

func (this *StateMachine) listenCandidateEventLoop() {

	votes := 1
	this.WriteLock.Lock()
	defer this.WriteLock.Unlock()
	this.StartElection()

	// 총 노드는 홀수 개 => 자신을 제외한 노드의 개수는 짝수
	majority := (len(this.Cluster.nodeAddressList) / 2) + 1
	timeoutChannel := time.After(MaxTimeout * 2)

	for this.GetStatus() == Candidate {
		select {

		case msg := <-*(this.ScheduleChannel):

			switch msg.Type {
			// Candidate가 되었다는 것은
			// 리더를 잃었다는 것이고
			// 따라서 Write operation 역시 미루어져야 한다.
			// Event Dispatcher 단에서 자신의 상태가 Candidate가 아닐 때까지
			// Lock을 걸어야한다.

			case VoteRequest:

				tools.InfoLogger.Printf(
					"나는 Candidate : 투표 독려 요청 옴, 내 term %d, 요청 term %d, 참가해야하나 ? %v",
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

				// 자신의 Term보다 높은 투표 요청이 온 경우
				// 투표 후, Follower가 된다.

				this.MetaDataLock.Lock()
				this.setTerm(msg.Term)
				this.MetaDataLock.Unlock()
				this.becomeFollower()

				(*msg.InterruptChannel) <- nil

			case ElectionResult:

				// Term이 같을 때도 포함하는 이유 :
				// 동일한 Term으로 Election이 동시에 일어났을 수도 있으므로
				//
				this.MetaDataLock.Lock()
				if msg.Term >= this.getTerm() {

					votes++

					if votes >= majority {
						this.broadcastElected()
						this.setNewLeader(
							this.Cluster.curIpAddress,
						)
						this.becomeLeader()
					}
				}
				this.MetaDataLock.Unlock()

			case Error:
				break

			case Heartbeat:

				// Candidate인 자신이, 리더만 보낼 수 있는 Heartbeat를 받았을 경우,
				// Term이 자신보다 크거나 '같을' 경우, 리더가 잘 선택된 것이다. => 자신이 Follower로 변경
				// Term이 자신과 같아도 Follower로 되야하는 이유는,
				// 동일한 Term의 Election이 여러개 발생할 수 있기 때문이다.
				this.MetaDataLock.Lock()

				if msg.Term >= this.getTerm() {
					this.setTerm(msg.Term)
					this.setNewLeader(msg.NewLeader)

					if msg.NewLeader != this.Cluster.curIpAddress {
						this.becomeFollower()
					}
				}

				this.MetaDataLock.Unlock()

				// 만약 리더로부터 받은 Heartbeat의 Term이 현재 Candidate인 나보다 작다면
				// Network split이 발생했다가 다시 합쳐진 것일 수도 있다
				// 그럴 경우, 자신의 선거는 계속 진행되며, 결과에 따라 Old Leader를 Follower로 변경하게 된다

				*(msg.InterruptChannel) <- nil

			}

		case <-timeoutChannel:

			tools.InfoLogger.Printf(
				"%d 번째 선거 타임아웃! - 개표 결과 투표수 : %d, 과반수 : %d",
				this.getTerm(),
				votes,
				majority,
			)

			// 선거 timeout이 끝났는데
			if votes >= majority {
				this.setNewLeader(
					this.Cluster.curIpAddress,
				)
				this.becomeLeader()
				this.broadcastElected()
				break
			}

			votes = 1
			this.StartElection()
			timeoutChannel = setMsgTimeOut()

		}

	}
}
