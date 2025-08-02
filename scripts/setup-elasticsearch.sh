#!/bin/bash

# Elasticsearch 인덱스 설정 스크립트

set -e

ELASTICSEARCH_URL="http://localhost:9200"

echo "🔍 Elasticsearch 연결 확인 중..."

# Elasticsearch 연결 대기
wait_for_elasticsearch() {
    local max_attempts=30
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        if curl -s "$ELASTICSEARCH_URL" > /dev/null 2>&1; then
            echo "✅ Elasticsearch 연결 성공"
            return 0
        fi
        
        echo "⏳ Elasticsearch 연결 대기 중... (시도 $attempt/$max_attempts)"
        sleep 2
        ((attempt++))
    done
    
    echo "❌ Elasticsearch 연결 실패"
    exit 1
}

# Elasticsearch 연결 대기
wait_for_elasticsearch

echo "📋 기존 인덱스 확인 및 삭제 중..."

# 기존 인덱스 삭제 (개발 환경용)
indices=("users" "social-graph" "recommendations")

for index in "${indices[@]}"; do
    if curl -s -o /dev/null -w "%{http_code}" "$ELASTICSEARCH_URL/$index" | grep -q "200"; then
        echo "🗑️  기존 인덱스 삭제: $index"
        curl -X DELETE "$ELASTICSEARCH_URL/$index" > /dev/null 2>&1
    fi
done

echo "🏗️  인덱스 템플릿 생성 중..."

# Users 인덱스 템플릿 생성
echo "📝 Users 인덱스 생성 중..."
curl -X PUT "$ELASTICSEARCH_URL/users" \
  -H "Content-Type: application/json" \
  -d '{
    "settings": {
      "number_of_shards": 3,
      "number_of_replicas": 1,
      "analysis": {
        "analyzer": {
          "username_analyzer": {
            "type": "custom",
            "tokenizer": "keyword",
            "filter": ["lowercase", "trim"]
          },
          "name_analyzer": {
            "type": "custom",
            "tokenizer": "standard",
            "filter": ["lowercase", "trim", "stop"]
          },
          "search_analyzer": {
            "type": "custom",
            "tokenizer": "standard",
            "filter": ["lowercase", "trim", "stop", "snowball"]
          }
        }
      }
    },
    "mappings": {
      "properties": {
        "user_id": {
          "type": "keyword",
          "index": true
        },
        "username": {
          "type": "text",
          "analyzer": "username_analyzer",
          "fields": {
            "keyword": {
              "type": "keyword"
            },
            "suggest": {
              "type": "completion",
              "analyzer": "username_analyzer"
            }
          }
        },
        "display_name": {
          "type": "text",
          "analyzer": "name_analyzer",
          "fields": {
            "keyword": {
              "type": "keyword"
            },
            "suggest": {
              "type": "completion",
              "analyzer": "name_analyzer"
            }
          }
        },
        "email": {
          "type": "keyword",
          "index": false
        },
        "bio": {
          "type": "text",
          "analyzer": "search_analyzer"
        },
        "location": {
          "properties": {
            "city": {
              "type": "keyword"
            },
            "state": {
              "type": "keyword"
            },
            "country": {
              "type": "keyword"
            },
            "coordinates": {
              "type": "geo_point"
            }
          }
        },
        "interests": {
          "type": "keyword",
          "fields": {
            "text": {
              "type": "text",
              "analyzer": "search_analyzer"
            }
          }
        },
        "social_metrics": {
          "properties": {
            "friend_count": {
              "type": "integer"
            },
            "mutual_friend_count": {
              "type": "integer"
            },
            "popularity_score": {
              "type": "float"
            },
            "activity_score": {
              "type": "float"
            },
            "response_rate": {
              "type": "float"
            }
          }
        },
        "privacy_settings": {
          "properties": {
            "searchable": {
              "type": "boolean"
            },
            "show_mutual_friends": {
              "type": "boolean"
            },
            "location_visible": {
              "type": "boolean"
            }
          }
        },
        "created_at": {
          "type": "date"
        },
        "updated_at": {
          "type": "date"
        },
        "last_active": {
          "type": "date"
        },
        "status": {
          "type": "keyword",
          "index": true
        }
      }
    }
  }'

echo ""
echo "📊 Social Graph 인덱스 생성 중..."
curl -X PUT "$ELASTICSEARCH_URL/social-graph" \
  -H "Content-Type: application/json" \
  -d '{
    "settings": {
      "number_of_shards": 5,
      "number_of_replicas": 1
    },
    "mappings": {
      "properties": {
        "relationship_id": {
          "type": "keyword"
        },
        "user_a": {
          "type": "keyword"
        },
        "user_b": {
          "type": "keyword"
        },
        "relationship_type": {
          "type": "keyword"
        },
        "strength_score": {
          "type": "float"
        },
        "created_at": {
          "type": "date"
        },
        "updated_at": {
          "type": "date"
        }
      }
    }
  }'

echo ""
echo "🎯 Recommendations 인덱스 생성 중..."
curl -X PUT "$ELASTICSEARCH_URL/recommendations" \
  -H "Content-Type: application/json" \
  -d '{
    "settings": {
      "number_of_shards": 3,
      "number_of_replicas": 1
    },
    "mappings": {
      "properties": {
        "user_id": {
          "type": "keyword"
        },
        "recommended_user_id": {
          "type": "keyword"
        },
        "recommendation_score": {
          "type": "float"
        },
        "mutual_friend_count": {
          "type": "integer"
        },
        "mutual_friends": {
          "type": "keyword"
        },
        "reason_code": {
          "type": "keyword"
        },
        "confidence_score": {
          "type": "float"
        },
        "algorithm_version": {
          "type": "keyword"
        },
        "created_at": {
          "type": "date"
        },
        "updated_at": {
          "type": "date"
        },
        "expires_at": {
          "type": "date"
        }
      }
    }
  }'

echo ""
echo "✅ Elasticsearch 인덱스 설정 완료!"

# 인덱스 상태 확인
echo ""
echo "📋 생성된 인덱스 목록:"
curl -s "$ELASTICSEARCH_URL/_cat/indices?v" | grep -E "(users|social-graph|recommendations)"

echo ""
echo "🎉 Elasticsearch 설정이 완료되었습니다!"