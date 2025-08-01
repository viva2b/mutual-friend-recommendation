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
├── cmd/                    # 애플리케이션 엔트리포인트
│   ├── server/            # gRPC 서버
│   └── batch/             # 배치 처리 애플리케이션
├── internal/              # 내부 비즈니스 로직
│   ├── api/               # gRPC API 구현
│   ├── batch/             # 배치 처리 로직
│   ├── domain/            # 도메인 모델
│   ├── repository/        # 데이터 액세스 계층
│   └── service/           # 비즈니스 서비스
├── pkg/                   # 공통 유틸리티
│   ├── config/            # 설정 관리
│   └── logger/            # 로깅
├── configs/               # 설정 파일
├── deployments/           # 배포 관련 파일
└── scripts/               # 유틸리티 스크립트
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