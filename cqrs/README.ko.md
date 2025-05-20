# Go.CQRS [![license](https://img.shields.io/badge/license-MIT-blue.svg?maxAge=2592000)](https://tictactoe/blob/master/LICENSE.md) [![Go Report Card](https://goreportcard.com/badge/github.com/jetbasrawi/go.cqrs)](https://goreportcard.com/report/github.com/jetbasrawi/go.cqrs) [![GoDoc](https://godoc.org/github.com/jetbasrawi/go.cqrs?status.svg)](https://godoc.org/github.com/jetbasrawi/go.cqrs)


## Golang CQRS 참조 구현체

Go.CQRS는 Golang에서 CQRS 구현을 지원하기 위한 인터페이스와 구현체를 제공합니다. examples 디렉토리에는 Go.CQRS 사용 방법을 보여주는 샘플 애플리케이션이 포함되어 있습니다.

Go.CQRS는 가능한 한 Greg Young이 주장하는 CQRS 원칙에 따라 설계되었으며, 이는 이 주제에 관한 최고의 사고 방식을 대표합니다.

## CQRS 패턴 vs CQRS 프레임워크

CQRS는 아키텍처 패턴입니다. CQRS 패턴을 구현할 때, 코드를 프레임워크로 패키징하는 것을 쉽게 상상할 수 있습니다. 그러나 CQRS를 다루는 사람들은 단순히 프레임워크를 사용하기보다는 패턴의 기본 세부 사항을 학습하는 데 집중하는 것이 좋습니다.

CQRS 패턴의 구현은 특별히 어렵지 않지만, 이 패턴이 전통적인 비 CQRS 아키텍처와 매우 다르기 때문에 학습 곡선이 가파릅니다. 애그리게이트 디자인과 같은 주제는 매우 다릅니다. 이벤트 소싱(EventSourcing)과 최종 일관성(eventual consistency)을 사용할 계획이라면 많은 학습이 필요합니다.

CQRS를 처음 접하거나 모범 사례에 관심이 있다면, Greg Young의 6시간짜리 [hands-on CQRS](https://www.youtube.com/watch?v=whCk1Q87_ZI) 워크숍 비디오가 큰 도움이 될 것입니다.

패턴을 이해한 후에는 Go.CQRS와 같은 구현체를 Golang에서 패턴을 구현하는 방법을 배우기 위한 참조로 사용할 수 있으며, CQRS 구현의 기반으로도 활용할 수 있습니다.

## Go.CQRS가 제공하는 것은 무엇인가요?

|기능|설명|
|-------|-----------|
| **Aggregate** | 애그리게이트에 필요한 공통 기능을 제공하기 위해 자신의 타입에 내장할 수 있는 AggregateRoot 인터페이스와 Aggregate 기본 타입 |
| **Event** | 이벤트 인터페이스와 이벤트를 위한 메시지 봉투인 EventDescriptor. Go.CQRS의 이벤트는 단순한 Go 구조체이며, 다른 Go 구현체에서처럼 매직 스트링이 없습니다. |
| **Command** | 커맨드 인터페이스와 커맨드를 위한 메시지 봉투인 CommandDescriptor. Go.CQRS의 커맨드는 단순한 Go 구조체이며, 다른 Go 구현체에서처럼 매직 스트링이 없습니다. | 
| **CommandHandler**| 커맨드 핸들러를 체이닝하기 위한 인터페이스와 기본 기능 |
| **Dispatcher** | 디스패처 인터페이스와 인메모리 디스패처 구현체 |
| **EventBus** | 이벤트 버스 인터페이스와 인메모리 구현체 |
| **EventHandler** | 이벤트 핸들러 인터페이스 |
| **Repository** | 리포지토리 인터페이스와 [GetEventStore](https://geteventstore.com/)에 이벤트를 저장하는 CommonDomain 리포지토리 구현체. MongoDB와 같은 일반 데이터베이스를 위한 많은 일반적인 이벤트 스토어 구현체가 있지만, [GetEventStore](https://geteventstore.com/)는 오픈 소스이며 성능이 좋고 이 분야에서 풍부한 경험을 가진 팀의 최고의 사고를 반영하는 특화된 이벤트 소싱 데이터베이스입니다. |
| **StreamNamer** | StreamNamer 인터페이스와 스트림 이름 지정에 유연성을 제공하기 위해 **func(string, string) string** 시그니처를 가진 함수 사용을 지원하는 DelegateStreamNamer 구현체. 일반적인 스트림 이름 구성 방법은 **BoundedContext** 이름에 AggregateID를 접미사로 사용하는 것입니다. | 

모든 구현체는 특정 요구 사항에 맞게 쉽게 대체할 수 있습니다.

## 예제 코드
examples 폴더에는 go.cqrs를 사용하여 서비스를 구성하는 방법에 대한 간단하고 명확한 예제가 포함되어 있습니다. 이 예제는 [Greg Young](https://github.com/gregoryyoung)의 고전적인 참조 구현체인 [m-r](https://github.com/gregoryyoung/m-r)을 포팅한 것입니다.

## 시작하기

```
    $ go get github.com/jetbasrawi/go.cqrs

```

Go.CQRS 사용 방법에 대한 지침은 예제 애플리케이션을 참조하세요.
