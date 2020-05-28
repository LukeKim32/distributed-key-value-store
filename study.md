# Redis_study
This repository is for studying Redis

Redis : "Re"mote "Di"ctionary "S"erver

1. Stores data as "Key:Value"

2. No-SQL : Use multiple data structures to store data (No tables or Select, Insert Query)

3. Interaction with data is "command based"

4. In-memory DB = Fast (Persistency Support)

Written in C

Why to use Redis?

Performance : https://redis.io/topics/benchmarks

Simple & Flexible : No need of defining tables, No Select/Insert/Update/Delete , With "Commands"

Durable : Option to write on disk(configurable), Used as Caching & Full-fledge db

Language support : https://redis.io/clients

Compatibility : Used as 2nd db to make transactions/queries faster

일반적인 DB와 request/response는 Disk에서 가져와 Time consuming
<->
Redis가 Cache 역할. 

Built-in Master-slave replication feature supported (replicate data to any slaves)

<img width="623" alt="스크린샷 2020-01-26 오후 8 12 36" src="https://user-images.githubusercontent.com/48001093/73134292-4076c480-4078-11ea-9cb8-e7c2d013d09a.png">

만약 Master가 죽으면, Slave가 자연스럽게 처리 가능(No downtime)

Single text file for all config

Single Threaded - One action at a time

Pipelining : cluster multiple command


==================

Redis Datatypes : Strings, Lists, Sets, Sorted Sets, Hashes, Bitmaps, Hyperlogs, Geospatial Indexes

MongoDB + memchached



====================
20. 2. 8 (Sat)

Redis는 기본적으로 Master-Slave replication이 제공된다.
이는 MySQL Master-Slave replication과 유사하다.

- 주요 특징
1. Master는 Write(&read), Slave는 Read only 이다.
2. One Master - Many Slaves
3. Asynchronous Replication
> 1) Send ACK to Client & 2) Propagate to Slave at the same time (Don't consider ACK from Slave)
If it's sync, Propagate to Slave => ACK from Slave => Send ACK to CLient
(This is a trade off for Performance with reliability)
4. If Master dies/fails, 1) Slave는 Master가 살아날 때까지 '주기적'으로 Connection 요청, 대기 (재연결 성공시 데이터 동기화) <br> 2) User가 ***수동으로*** master, slave node switch 해야 한다.

- 문제점
1. 저장하려는 데이터가 Master 레디스의 가능한 메모리 용량보다 크다 (Auto sharding이 안되므로)
2. Master 노드가 문제가 발생하면, Application의 작동을 위해, 노드를 수동으로 바꾸어주어야 한다.
3. Master가 fail이 발생할 경우, Read는 Slave를 통해 여전히 가능하지만, Write가 불가능하다(a.k.a "Low Availability")
3. 여러 노드에 데이터 분배가 자동으로 되지 않는다.

=> Redis를 개발한 Salvatore Sanfilippo가 이 문제점들을 해결하기 위해 **Redis Sentinel**을 개발하게 되었다.

### Redis Sentinel

- 주요 특징 :
1. ***Automatically*** Promotes one of Slaves to Master, if Master fails
Master의 Downtime을 최소화한다(새로 Master를 올리므로, a.k.a "High Availability")
2. 데이터 분산(Sharding)은 제공하지 않는다.
3. 레디스 서버 자체와는 별개의 독립 프로세스(독립 포트)
4. 안정적인 운영을 위해서는 최소 3개 이상의 센티널 인스턴스가 필요 (fail over decision을 위해 과반수 이상의 vote 필요)
> 일반적으로 1 redis server - 1 redis sentinel <br> <img width="349" alt="스크린샷 2020-02-08 오후 4 05 30" src="https://user-images.githubusercontent.com/48001093/74080971-dfde8300-4a8c-11ea-9d08-411eeb654e7b.png">

5. 클라이언트는 Redis server가 아닌 Sentinel에 데이터를 쿼리
4. 기존 레디스 인터페이스와 달라 레디스 센티널을 지원하는 레디스 클라이언트를 따로 이용해야한다.

- 문제점
1. 데이터 분산(Sharding)을 제공하지 않는다.

2. Master가 변경된 경우(ex. Slave가 승격), Client는 이를 자동으로 감지할 방법이 없기에, Reverse Proxy를 이용해 Port forwarding을 한다. (단 Reverse Proxy가 죽는 경우, Redis 전체를 사용하지 못할 수 있기에, 여러 대의 Proxy 서버를 둘 수도 있다.)
<img width="353" alt="스크린샷 2020-02-08 오후 4 11 36" src="https://user-images.githubusercontent.com/48001093/74081031-b1ad7300-4a8d-11ea-9fc4-849e11fb0b97.png">
3. Network Partition이 발생한 경우, 데이터 손실 발생(데이터 분산을 제공하지 않기 때문에)

> ex. 초기 상황 : <br> <img width="353" alt="스크린샷 2020-02-08 오후 4 26 32" src="https://user-images.githubusercontent.com/48001093/74081208-c7239c80-4a8f-11ea-8032-c0e4150e4587.png">

> 네트워크 파티션이 발생한 경우, 슬레이브 중 하나가 마스터가 될 것이고(a.k.a "Split brain") <br> <img width="348" alt="스크린샷 2020-02-08 오후 4 26 55" src="https://user-images.githubusercontent.com/48001093/74081210-d4d92200-4a8f-11ea-86f8-2539e75e4fda.png"> <br> 클라이언트는 Slave가 붙어있지 않은 마스터에 Write를 계속할 것이다. 

> 네트워크 파티션이 해결된다면, "대부분의 센티널은 기존 마스터가 새로운 마스터(격리되어 승격했던 예전 Slave)의 슬레이브가 되는 것에 동의"(*Todo : Master를 결정하는 알고리즘*), 데이터 분산 기능이 없기에, 그동안 레디스 클라이언트가 쓴 모든 작업은 사라질 것이다.

=> 데이터 분산을 해결하기 위해 **Redis Cluster**가 등장

### Redis Cluster

- 주요 특징 :
1. 하나의 프로세스만 필요(각 레디스 서버 하나 당 한 대 씩 유지하는 Sentinel과 달리)
2. Redis 노드들 내부 통신용 Port를 추가 Open (내부 통신 : Error, Fail, Resharding)
3. Failure 처리 : Redis 노드들끼리 ***매 초 패킷 교환(Ping-pong, Heartbeat)***

> 패킷에 몇가지 정보를 담아 보낸다(ex. Failover 투표 요청, 전송 노드의 Bitmap hash slot)

> 노드는 소수의 랜덤 노드들에게 매초 ping 패킷을 보내, 전체 전송된 ping 패킷은 클러스터 내 노드의 개수에 상관없이 일정한 수를 유지하도록 한다. <br> 그러나, 설정한 "NODE_TIMEOUT" 시간의 반이 넘어가지 않게 한 노드에서 ping 패킷을 보내지 않은 노드는 없도록 한다. (NODE_TIMOUT이 넘어가기 전에 TCP link reconnect를 시도하기도 한다) <br> 이러한 정책으로, NODE_TIMEOUT 시간은 작은데, Cluster 내 노드의 개수가 많을 경우, Ping 패킷의 개수가 굉장히 많아질 수도 있다. (그래도 아직까지 패킷 개수 이슈는 없었다)

4. 모든 데이터는 마스터 단위로 분산, 슬레이브 단위로 복제
4. Slave가 존재할 시, ***Automatically*** 승격이 가능하나 Latency가 존재한다.
(새로운 Master가 나타나기 전까지 - 기존 Fail Master의 Slot이 정상적인 Node에 다시 할당되기 전까지 - 모든 Node들은 Query를 받지 않는다)

- 문제점 :
1. 슬레이브가 즉각 Master 승격이 이루어지지 않는다.
(Cluster의 승격 프로세스 : Connection loss감지 => 모든 노드들이 Fail agree(과정 정리해야함) => Cluster state 변화(ok->fail) => Failover election으로 Slave가 New master => Cluster state 변화(fail->ok))

> Cluster를 구성하는 Node가 하나라도 fail하면 Cluster state = fail 인데, 다른 살아있는 노드들도 더이상 입력을 받지 못한다. <br> fail Node가 제거되어도, state는 여전히 fail 상태인데, fail Node의 slot을 다시 할당해주어야 state가 ok로 변한다.
(수동 작업 가능)

2. Initial Slot distribute 이후에는, 마스터에 대한 Slot 할당은 모두 수동이다.

3. Master, Slave가 모두 죽으면, 해당 데이터를 모두 잃게 된다.

### 개발 목표
(With only using basic Redis Server process - using only (Key,Value) in memory store feature)

* **Sprint #1 Simple Redis Cluster**
  1. sharding data query
  2. Master/Slave replication
  3. Promote Slave to Master, if Master fails

* **Sprint #2 Advancement**
  1. Initial Slot distribution이후, Master에 대한 Slot 할당 자동화
  2. Masger, Slave가 모두 죽으면, 해당 데이터를 다른 노드로 옮기기

궁금한 점:
1. Sentinel과 Cluster의 Promoting-master 시간의 차이
2. 각 Heartbeat, Vote의 알고리즘 구체적으로 정리해보기
