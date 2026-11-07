#!/usr/bin/env python3
"""Create private runtime or test configuration without changing an existing one."""
from pathlib import Path
import argparse,secrets

parser=argparse.ArgumentParser();parser.add_argument('--test',action='store_true');args=parser.parse_args()

root = Path(__file__).resolve().parents[1]
secret_dir=root/'.secrets';secret_dir.mkdir(mode=0o700,exist_ok=True);secret_dir.chmod(0o700)
root_password=secret_dir/'mysql-root'
if not root_password.exists():
    with root_password.open('x') as handle:
        root_password.chmod(0o600);handle.write(secrets.token_hex(32)+'\n')
target = root / ('.env.test' if args.test else '.env')
if target.exists():
    print('沿用已有本机 '+target.name)
    raise SystemExit(0)
values = dict(
    COMPOSE_PROJECT_NAME='campusclaw-team-async', APP_VERSION='async-dev',
    API_PORT='8081', WEB_PORT='18080', MYSQL_HOST='db', MYSQL_PORT='3306',
    MYSQL_DATABASE='campusclaw', MYSQL_USER='campusclaw', MYSQL_PASSWORD=secrets.token_hex(24),
    SESSION_SECRET=secrets.token_hex(32), SEED_TEACHER_PASSWORD=secrets.token_hex(12),
    SEED_STUDENT_PASSWORD=secrets.token_hex(12), UPLOAD_DIR='/app/uploads', MAX_UPLOAD_BYTES='1048576',
    SESSION_TTL_HOURS='24', LOGIN_FAIL_LIMIT='50', LOGIN_FAIL_WINDOW_SECONDS='300',
    EMBEDDING_BASE_URL='http://gateway:8089/v1', EMBEDDING_API_KEY=secrets.token_hex(16),
    EMBEDDING_MODEL='fixture-embedding', EMBEDDING_DIM='8', CHAT_BASE_URL='http://gateway:8089/v1',
    CHAT_API_KEY=secrets.token_hex(16), CHAT_MODEL='fixture-chat', QDRANT_URL='http://qdrant:6333',
    CHUNK_AUTO_MAX_CHARS='800', CHUNK_AUTO_OVERLAP='80', VECTOR_SCORE_THRESHOLD='0.25',
    RRF_K='60', ASK_TOP_K='4', MCP_AUTH_TOKEN=secrets.token_hex(24),
    MCP_ENCRYPTION_KEY=secrets.token_hex(32), MCP_ALLOWED_BASE_URLS='http://mcp-demo:8090',
    MCP_MAX_ROUNDS='3', MCP_MAX_CALLS_PER_TURN='6', MCP_MAX_CALLS_PER_CONVERSATION='30',
    RABBITMQ_USER='campusclaw', RABBITMQ_PASSWORD=secrets.token_hex(24),
    GRADE_TIMEOUT_SECONDS='5', GRADE_MAX_ATTEMPTS='3', GRADE_LEASE_SECONDS='8',
    GRADE_RETRY_BASE_MS='1000', OUTBOX_POLL_MS='200',
)
if args.test:
    values.update(COMPOSE_PROJECT_NAME='campusclaw-team-test',WEB_PORT='18081',CAMPUSCLAW_ENV_FILE='.env.test')
else:
    values.update(CHAT_BASE_URL='',CHAT_API_KEY='',CHAT_MODEL='',GRADE_TIMEOUT_SECONDS='60',GRADE_LEASE_SECONDS='70')
values['AMQP_URL'] = 'amqp://' + values['RABBITMQ_USER'] + ':' + values['RABBITMQ_PASSWORD'] + '@rabbitmq:5672/'
with target.open('x') as handle:
    target.chmod(0o600)
    handle.write(''.join(f'{key}={value}\n' for key,value in values.items()))
print('已生成'+target.name+('隔离测试配置' if args.test else '；启动前填写CHAT_BASE_URL、CHAT_API_KEY、CHAT_MODEL')+'；口令仅保存在本机')
