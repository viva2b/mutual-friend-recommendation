# Mutual Friend System

친구 관계 시뮬레이션 프로젝트 - RabbitMQ와 고루틴 파이프라인을 활용한 이벤트 기반 배치 처리 시스템

## 프로젝트 개요

이 프로젝트는 **RabbitMQ와 고루틴 파이프라인**을 활용한 이벤트 기반 배치 처리와 **DynamoDB**, **Elasticsearch**를 중심으로 한 친구 관계 시뮬레이션 시스템입니다.

### 핵심 기능
- 친구 추가/삭제
- 친구 목록 조회  
- 사용자 검색
- 추천 친구 목록 조회

### 주요 아키텍처
- **CQRS** (Command Query Responsibility Segregation)
- **Event-Driven Architecture (EDA)**
- **Micro-Batch & Offline Batch Processing**

## 기술 스택

- **언어**: Go (Golang)
- **API**: gRPC with Protocol Buffers
- **데이터베이스**: Amazon DynamoDB (로컬 환경)
- **검색 엔진**: Elasticsearch (로컬 환경)  
- **메시지 큐**: RabbitMQ
- **캐시**: Redis

## 프로젝트 구조

```
mutual-friend/
├── cmd/                          # 실행 가능한 애플리케이션
│   ├── server/                   # 메인 gRPC 서버
│   ├── setup/                    # 데이터베이스 초기화
│   ├── comprehensive-test/       # 통합 gRPC API 테스트
│   ├── elasticsearch-*/          # Elasticsearch 설정 및 고급 테스트
│   ├── test-client/              # 기본 클라이언트 테스트
│   ├── test-cache-*/             # 캐시 성능 및 통합 테스트
│   ├── grpc-client/              # gRPC 클라이언트 도구
│   ├── search-benchmark/         # 검색 성능 벤치마크
│   └── query/                    # 데이터 쿼리 도구
├── internal/                     # 프라이빗 애플리케이션 코드
│   ├── api/                      # gRPC API 핸들러
│   ├── service/                  # 비즈니스 로직
│   ├── repository/               # 데이터 접근 계층
│   ├── search/                   # Elasticsearch 검색 서비스
│   ├── cache/                    # 캐시 서비스
│   └── domain/                   # 도메인 엔티티
├── pkg/                          # 재사용 가능한 라이브러리
│   ├── config/                   # 설정 관리
│   ├── cache/                    # 캐시 인터페이스 및 타입
│   ├── events/                   # 이벤트 시스템
│   ├── redis/                    # Redis 클라이언트
│   ├── elasticsearch/            # Elasticsearch 클라이언트
│   └── rabbitmq/                 # RabbitMQ 클라이언트
├── tests/                        # 종합 테스트 스위트 (8개 카테고리)
│   ├── integration/              # 통합 테스트
│   ├── performance/              # 성능 테스트
│   ├── stress/                   # 스트레스 테스트
│   ├── concurrency/              # 동시성 테스트
│   ├── consistency/              # 일관성 테스트
│   ├── architecture/             # 아키텍처 테스트
│   └── optimization/             # 최적화 테스트
├── scripts/                      # 자동화 스크립트
└── configs/                      # 설정 파일들
```

## 개발 환경 설정

### 1. 필수 요구사항
- Go 1.21+
- Docker & Docker Compose
- Protocol Buffers compiler (protoc)

### 2. 환경 설정

```bash
# 의존성 설치
make install-deps

# protoc 및 Go 플러그인 설치 (macOS)
make proto-install

# 개발 환경 시작 (Docker 서비스)
make dev
```

### 3. 서비스 접속 정보
- **DynamoDB**: http://localhost:8000
- **DynamoDB Admin**: http://localhost:8001
- **RabbitMQ Management**: http://localhost:15672 (admin/admin123)
- **Elasticsearch**: http://localhost:9200
- **Redis**: localhost:6379
- **Redis Commander**: http://localhost:8081

## 사용 가능한 명령어

```bash
make help           # 모든 명령어 보기
make build          # 애플리케이션 빌드
make test           # 테스트 실행
make docker-up      # Docker 서비스 시작
make docker-down    # Docker 서비스 중지
make proto          # Protocol Buffers 파일 생성
make run-server     # gRPC 서버 실행
make run-batch      # 배치 프로세서 실행
```

## 라이선스

MIT License