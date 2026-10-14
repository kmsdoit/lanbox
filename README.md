# lanbox

LAN IP와 SSH가 준비된 OS 컨테이너를 만들고 지우는 CLI다.

`lanbox create dev1 --os ubuntu:24.04` 한 줄로 컨테이너를 띄우고, `lanbox rm dev1` 한 줄로 지운다. 상태는 Docker 라벨에만 둔다. `lanbox.managed=true`가 없는 리소스는 삭제하지 않는다.

대상은 Linux 호스트의 Docker 엔진이다. Wi-Fi 부모 인터페이스는 ipvlan, 유선은 macvlan을 쓴다. Windows, macOS, BSD 컨테이너와 Docker Desktop은 범위 밖이다.

## 빌드

저장소 루트에서:

```bash
go build -o bin/lanbox ./cmd/lanbox
```

릴리스 바이너리와 `.deb`는 goreleaser 설정(`.goreleaser.yaml`)으로 만든다. 대상은 linux/amd64, linux/arm64다.

```bash
goreleaser release --snapshot --clean
```

모듈 경로에서 바로 설치할 수도 있다.

```bash
go install github.com/kmsdoit/lanbox/cmd/lanbox@latest
```

## 설치

GitHub Releases에 linux/amd64, linux/arm64 바이너리(`tar.gz`)와 `.deb`가 있다. 받은 `.deb`는 `dpkg`로 설치한다. 패키지는 `/usr/local/bin/lanbox`에 둔다.

```bash
sudo dpkg -i lanbox_<version>_linux_amd64.deb
```

arm64 호스트는 `lanbox_<version>_linux_arm64.deb`를 쓴다.

바이너리만 받을 때는 아카이브를 푼 뒤 같은 경로에 둔다.

```bash
tar -xzf lanbox_<version>_linux_amd64.tar.gz
sudo install -m 755 lanbox /usr/local/bin/lanbox
```

## 최초 준비

네트워크 생성, shim, ARP 검사에는 root 또는 `CAP_NET_ADMIN` / `CAP_NET_RAW`가 필요하다. 인스턴스 생성과 조회는 docker 그룹으로 충분하다.

`init`은 부모 인터페이스, 서브넷, 게이트웨이를 감지한다. Wi-Fi면 ipvlan, 유선이면 macvlan을 제안한다. 감지가 실패하면 값을 직접 지정한다. `--parent`에는 `ip -br addr`에 나온 이름을 쓴다. 아래는 예시 `eth0`이다.

```bash
sudo lanbox init --parent eth0 --subnet 192.168.1.0/24 --gateway 192.168.1.1 --driver ipvlan --range 192.168.1.64/28 --shim --reserve 192.168.1.70,192.168.1.10
```

예시를 그대로 적용하면 네트워크 `lanbox_net`, ipvlan, parent `eth0`, subnet `192.168.1.0/24`, gateway `192.168.1.1`, range `192.168.1.64/28`이다. shim 주소는 범위 안에서 고르고, reserve에는 그 주소와 게이트웨이, 호스트 주소를 넣는다. 기본 계정은 `lanbox`, 모드는 `light`다. 설정은 `/etc/lanbox/config.yaml`에 저장하고, 같은 내용을 Docker 네트워크 라벨에도 남긴다. 파일이 없어도 라벨로 복구한다.

호스트 LAN과 SSH는 부모 인터페이스로 나가고, 컨테이너 대역만 shim으로 나간다. 예: `192.168.1.0/24 dev eth0`, `192.168.1.64/28 dev lanbox-shim`. shim이 부모와 같은 `192.168.1.0/24`를 더 우선하면 호스트 주소가 두 개라 `192.168.1.10` SSH가 됐다 안 됐다 한다. 재부팅 후에는 `sudo lanbox shim up`이 필요하다. 부모가 무선이면 ipvlan 지연이 흔들릴 수 있다. 유선은 macvlan이 기본이다.

다른 경로를 쓰려면 `--config-dir` 또는 `LANBOX_CONFIG_DIR`를 준다.

호스트 주소가 DHCP라면 공유기에 고정 할당하는 것을 권한다. 주소가 바뀌면 shim과 접속 경로가 깨진다. 할당 범위는 DHCP 풀과 겹치지 않게 잡고, 같은 LAN의 호스트끼리는 비어 있는 구간을 나눈다. 한 호스트가 `192.168.1.64/28`이면 다른 호스트는 `192.168.1.80/28`처럼 겹치지 않는 범위를 쓴다.

## 인스턴스

```bash
lanbox create dev1 --os ubuntu:24.04 --cpus 2 --memory 2g
lanbox create web  --os rocky -n 3 --cpus 1 --memory 1g
lanbox create edge --os alpine

lanbox ls
lanbox ssh web-2
lanbox shell web-2

lanbox rm dev1
lanbox rm -g web
lanbox purge -y
```

`-n 3`이면 `web-1`부터 `web-3`까지 한 그룹으로 만든다. 하나라도 실패하면 그 실행에서 만든 컨테이너와 새 볼륨만 롤백한다. 접속은 `ssh lanbox@<ip>` 형태다.

할당 범위를 `192.168.1.64/28`(64–79)로 잡았다고 하면, 네트워크 주소 `.64`와 브로드캐스트 `.79`는 할당하지 않고 앞에서부터 빈 주소를 고른다. 예약과 shim으로 잡은 주소는 건너뛴다. 게이트웨이 `.1`과 호스트 `.10`은 이 /28 밖이다.

`--os`는 필수다. 별칭(`ubuntu`, `rocky`) 또는 이미지(`ubuntu:24.04`, `myrepo/custom:1.0`)다. 별칭은 `/etc/lanbox/os.yaml`과 내장 기본값이다. 사내 이미지는 `lanbox os add lab myrepo/custom:1.0`로 등록한다.

계열은 이미지의 `/etc/os-release`로 정한다. debian, rhel, alpine, suse, arch를 지원한다. 감지에 실패하면 `--family`로 지정한다.

파생 이미지 태그는 `lanbox/<os>-<tag>:<해시>`다. 해시는 계열 템플릿, 계정, 추가 패키지, 비밀번호 로그인 여부, 모드를 포함한다. 같은 조합의 두 번째 생성은 빌드를 건너뛴다.

기본 계정은 `lanbox`이고 sudo가 된다. `--user`를 빼면 이 값을 쓴다. SSH는 공개키만 허용한다. 비밀번호는 `--password`를 넣은 경우만 켜지고, root 로그인은 꺼진다. 공개키는 `--ssh-key`, 그다음 설정의 `ssh_key`, 그다음 `~/.ssh/id_ed25519.pub`, `id_rsa.pub`, `id_ecdsa.pub` 순으로 찾는다.

`--memory-swap`은 메모리 제한과 같게 고정한다. `--home-volume`은 `/home`을 볼륨에 둔다. `rm`은 컨테이너만 지우고 그 볼륨은 남긴다. `purge`가 볼륨까지 지운다.

`--gpus all` 또는 `--gpus 1`로 NVIDIA GPU를 요청한다. 랩 스펙의 `gpus`도 같다. 생략하면 붙이지 않고, 생성할 때만 지정한다.
`--gpus`는 NVIDIA GPU와 `docker info` 런타임 `nvidia`가 있어야 한다. 확인은 `docker inspect <name> --format '{{json .HostConfig.DeviceRequests}}'`와 컨테이너 안 `nvidia-smi`다.

`rm`과 `purge`는 대상을 보여 주고 확인을 받는다. `-y`와 `--dry-run`이 있다. 터미널이 아니면 `-y`가 필요하다.

## 운영 명령

| 명령 | 하는 일 |
| --- | --- |
| `lanbox start\|stop\|restart <name>` | 상태 변경 |
| `lanbox resize <name> --cpus --memory` | 실행 중 자원 변경 |
| `lanbox info <name>` | 접속 정보, 제한, 사용량, 호스트 커널 |
| `lanbox doctor` | 자원, IP, 무선, shim, DHCP 점검 |
| `lanbox stats` | 실사용량 |
| `lanbox os ls` / `os add` | 별칭 |
| `lanbox image prune` | 쓰지 않는 파생 이미지 |
| `lanbox shim up\|down\|install` | shim 생성, 삭제, systemd 등록 |
| `lanbox snapshot <name>` | `docker commit`. `create --from <snapshot>`으로 복제 |
| `lanbox apply -f lab.yaml` | 스펙과 차이를 반영 |

`info`의 커널은 호스트 값이다. 컨테이너는 호스트 커널을 공유한다.

`apply`는 개수 변경을 생성·삭제로, CPU·메모리를 `docker update`로, OS·패키지·모드·계정·IP 변경을 재생성으로 반영한다. 스펙에 없는 다른 인스턴스는 지우지 않는다. 그룹 개수가 줄면 그 그룹의 남는 번호만 지운다.

`--mode systemd`는 `/sbin/init`을 PID 1로 두고 cgroup 마운트와 추가 권한이 필요하다. alpine은 systemd 모드를 지원하지 않는다. 기본은 `light`이며 PID 1은 tini, 그 다음이 `sshd -D`다.

`--output json`은 표 대신 JSON을 출력한다.

## 패키지 이름

추가 패키지는 해당 계열 패키지 매니저에 그대로 전달한다. 아래 이름은 계열에 맞게 바꾼다.

| 입력 | debian | rhel | alpine / suse / arch |
| --- | --- | --- | --- |
| `iproute2` | iproute2 | iproute | iproute2 |
| `iputils-ping` | iputils-ping | iputils | iputils |
| `procps` | procps | procps-ng | procps |

빌드가 실패하면 계열과 패키지 이름을 오류에 담는다.

## 테스트

```bash
go test ./...
```

실제 이미지로 ubuntu, rocky, alpine을 띄우고 다른 PC에서 SSH한 뒤 `purge`로 잔여가 0인지 보는 확인은 Linux 서버와 LAN이 필요하다. `LANBOX_E2E=1`이 아니면 `test/e2e`는 건너뛴다.

## 설정 예

`/etc/lanbox/config.yaml` 예시:

```yaml
network:
  name: lanbox_net
  driver: ipvlan
  parent: eth0
  subnet: 192.168.1.0/24
  gateway: 192.168.1.1
  range: 192.168.1.64/28
shim:
  enabled: true
  ip: 192.168.1.78
reserve:
  - 192.168.1.70
  - 192.168.1.10
  - 192.168.1.78
  - 192.168.1.1
defaults:
  user: lanbox
  ssh_key: ~/.ssh/id_ed25519.pub
  mode: light
```

랩 스펙:

```yaml
instances:
  - name: dev1
    os: ubuntu:24.04
    cpus: 2
    memory: 2g
    gpus: all
    packages: [vim, git]
  - name: web
    os: rocky
    count: 3
    cpus: 1
    memory: 1g
  - name: edge
    os: alpine
    ip: 192.168.1.72
```

## 트러블슈팅

부모 인터페이스 이름을 잘못 주면 네트워크 생성이 실패한다. `--parent`에는 `ip -br addr`에 나온 이름을 쓴다. 예시는 `eth0`이다. `eth0`이 아닌데 `eth0`을 넣으면 Docker가 `invalid subinterface vlan name eth0, example formatting is eth0.10`처럼 실패할 수 있다. 이 경우 설정 파일이 기록되기 전에 멈추므로, 올바른 `--parent`로 `sudo lanbox init`을 다시 실행하면 된다.

같은 LAN의 호스트끼리 할당 범위가 겹치면 안 된다.

호스트 SSH가 간헐적이면 `ip route`에 shim의 넓은 서브넷 경로가 있는지 본다. 호스트 LAN은 부모로, 컨테이너 대역만 shim으로 나간다. 재부팅 후에는 `sudo lanbox shim up`이 필요하다.

무선 ipvlan은 지연이 흔들릴 수 있다. 유선 기본은 macvlan이다. macvlan은 컨테이너 MAC이 부모와 달라서, 스위치가 포트당 MAC을 하나만 허용하면 호스트 shim에서는 접속되고 다른 PC에서는 안 된다. promiscuity가 이미 있어도 밖에서는 안 될 수 있다. 그때는 `--driver ipvlan`으로 다시 만든다. ipvlan L2는 컨테이너가 부모 MAC을 쓴다. 다른 PC의 ARP가 부모 MAC이면 밖으로 나가는 것이다.

`lanbox create`가 `차단: arp: LAN에서 이미 응답하는 IP가 있습니다`로 멈추면 그 주소는 다른 장비가 쓰는 것이다. `--ip`로 다른 빈 주소를 지정한다. 오래된 이웃 항목이면 `ip neigh del <ip> dev eth0` 후 다시 probe 한다. 응답이 계속이면 그 주소는 건너뛴다.

`ssh lanbox@<ip>`가 `Permission denied (publickey)`이면 생성 때 `--password`가 없어서 `PasswordAuthentication no`로 빌드된 것이다. 새로 만들 때는 `--password`를 준다.

`--gpus`는 NVIDIA GPU와 `docker info` 런타임 `nvidia`가 있어야 한다. 없으면 `lanbox create`가 실패한다. 확인은 `docker inspect`의 `DeviceRequests`와 `nvidia-smi`다.

호스트의 `hostname -I`에 컨테이너 IP가 없는 것은 정상이다. 컨테이너 주소는 `lanbox ls`로 본다.

`docker logs -f`는 터미널을 붙잡으므로 상태 확인 후 Ctrl+C로 나온다.
