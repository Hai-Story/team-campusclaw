// Package config 从环境变量读取全部配置。
// 设计约束（design.md D5）：密钥与凭据只来自环境变量，缺失即启动失败，禁止内置默认值。
package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"time"
)

// Config 是应用的全部运行配置。
type Config struct {
	Version             string
	Port                string        // api 监听端口（Compose 内部，不暴露宿主）
	UploadDir           string        // 上传目录（只挂载到 api 容器）
	SessionSecret       string        // 会话 ID 的 HMAC 密钥（必填，无默认值）
	MySQLHost           string        // 数据库主机（Compose 内部网络服务名）
	MySQLPort           string        // 数据库端口
	MySQLDatabase       string        // 数据库名（必填）
	MySQLUser           string        // 普通数据库账号（必填；不下发 root 口令）
	MySQLPassword       string        // 普通账号口令（必填）
	SeedTeacherPassword string        // 预置教师口令（必填，bcrypt 入库）
	SeedStudentPassword string        // 预置学生口令（必填，bcrypt 入库）
	MaxUploadBytes      int64         // 上传大小上限
	SessionTTL          time.Duration // 会话有效期
	LoginFailLimit      int           // 登录失败限流阈值
	LoginFailWindow     time.Duration // 登录失败统计窗口

	// --- 知识库检索（嵌入/对话网关 + Qdrant），密钥缺失即启动失败 ---
	EmbeddingBaseURL  string  // 嵌入网关地址（只到 /v1，程序自行接 /embeddings）
	EmbeddingAPIKey   string  // 嵌入网关密钥（必填，不下发浏览器）
	EmbeddingModel    string  // 嵌入模型名
	EmbeddingDim      int     // 向量维度（Qdrant 集合创建时使用）
	ChatBaseURL       string  // 对话网关地址（问答仅在检索命中后调用）
	ChatAPIKey        string  // 对话网关密钥（必填）
	ChatModel         string  // 对话模型名
	QdrantURL         string  // Qdrant 地址（Compose 内部网络）
	ChunkAutoMaxChars int     // auto 切分窗口（默认 800）
	ChunkAutoOverlap  int     // auto 切分重叠（默认 80）
	VectorScoreCutoff float64 // 向量相似度丢弃阈值（默认 0.35）
	RRFK              int     // RRF 融合常数 k（默认 60）
	AskTopK           int     // 问答取前 N 条切片（默认 4）
}

// required 返回环境变量值；缺失或为空时报错（失败即停，不回落默认值）。
func required(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("缺少必需环境变量 %s（请参照 .env.example 填写 .env）", name)
	}
	return v, nil
}

func getenvInt64(name string, def int64) (int64, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("环境变量 %s 必须为正整数，当前值 %q", name, v)
	}
	return n, nil
}

func getenvFloat(name string, def float64) (float64, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 0 || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("环境变量 %s 必须为非负小数，当前值 %q", name, v)
	}
	return f, nil
}

// Load 读取并校验配置；任一必填项缺失即返回错误，进程应退出。
func Load() (*Config, error) {
	c := &Config{}
	var err error
	for _, key := range []string{"APP_VERSION", "API_PORT", "WEB_PORT", "MYSQL_HOST", "MYSQL_PORT", "MYSQL_DATABASE", "MYSQL_USER", "MYSQL_PASSWORD", "SESSION_SECRET", "SEED_TEACHER_PASSWORD", "SEED_STUDENT_PASSWORD", "UPLOAD_DIR", "MAX_UPLOAD_BYTES", "SESSION_TTL_HOURS", "LOGIN_FAIL_LIMIT", "LOGIN_FAIL_WINDOW_SECONDS", "EMBEDDING_BASE_URL", "EMBEDDING_API_KEY", "EMBEDDING_MODEL", "EMBEDDING_DIM", "CHAT_BASE_URL", "CHAT_API_KEY", "CHAT_MODEL", "QDRANT_URL", "CHUNK_AUTO_MAX_CHARS", "CHUNK_AUTO_OVERLAP", "VECTOR_SCORE_THRESHOLD", "RRF_K", "ASK_TOP_K", "MCP_AUTH_TOKEN", "MCP_ENCRYPTION_KEY", "MCP_ALLOWED_BASE_URLS", "MCP_MAX_ROUNDS", "MCP_MAX_CALLS_PER_TURN", "MCP_MAX_CALLS_PER_CONVERSATION"} {
		if _, err := required(key); err != nil {
			return nil, err
		}
	}
	c.Version = os.Getenv("APP_VERSION")
	if len(c.Version) > 64 {
		return nil, fmt.Errorf("APP_VERSION 长度须为1至64")
	}
	if len(os.Getenv("SESSION_SECRET")) < 32 {
		return nil, fmt.Errorf("SESSION_SECRET 至少32字节")
	}
	for _, key := range []string{"API_PORT", "WEB_PORT", "MYSQL_PORT"} {
		n, err := strconv.Atoi(os.Getenv(key))
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("%s 须为1到65535的整数", key)
		}
	}

	if c.SessionSecret, err = required("SESSION_SECRET"); err != nil {
		return nil, err
	}
	if c.MySQLDatabase, err = required("MYSQL_DATABASE"); err != nil {
		return nil, err
	}
	if c.MySQLUser, err = required("MYSQL_USER"); err != nil {
		return nil, err
	}
	if c.MySQLPassword, err = required("MYSQL_PASSWORD"); err != nil {
		return nil, err
	}
	if c.SeedTeacherPassword, err = required("SEED_TEACHER_PASSWORD"); err != nil {
		return nil, err
	}
	if c.SeedStudentPassword, err = required("SEED_STUDENT_PASSWORD"); err != nil {
		return nil, err
	}

	// 非密钥项允许默认值（不构成安全问题）。
	c.Port = os.Getenv("API_PORT")
	if c.Port == "" {
		c.Port = "8081"
	}
	c.MySQLHost = os.Getenv("MYSQL_HOST")
	if c.MySQLHost == "" {
		c.MySQLHost = "db"
	}
	c.MySQLPort = os.Getenv("MYSQL_PORT")
	if c.MySQLPort == "" {
		c.MySQLPort = "3306"
	}
	c.UploadDir = os.Getenv("UPLOAD_DIR")
	if c.UploadDir == "" {
		c.UploadDir = "./uploads"
	}

	if c.MaxUploadBytes, err = getenvInt64("MAX_UPLOAD_BYTES", 1<<20); err != nil {
		return nil, err
	}
	ttlHours, err := getenvInt64("SESSION_TTL_HOURS", 24)
	if err != nil {
		return nil, err
	}
	c.SessionTTL = time.Duration(ttlHours) * time.Hour
	limit, err := getenvInt64("LOGIN_FAIL_LIMIT", 5)
	if err != nil {
		return nil, err
	}
	c.LoginFailLimit = int(limit)
	windowSec, err := getenvInt64("LOGIN_FAIL_WINDOW_SECONDS", 300)
	if err != nil {
		return nil, err
	}
	c.LoginFailWindow = time.Duration(windowSec) * time.Second

	// --- 网关密钥必填（缺失即启动失败，design.md D3 / spec 配置与密钥边界） ---
	if c.EmbeddingBaseURL, err = required("EMBEDDING_BASE_URL"); err != nil {
		return nil, err
	}
	if c.EmbeddingAPIKey, err = required("EMBEDDING_API_KEY"); err != nil {
		return nil, err
	}
	if c.ChatBaseURL, err = required("CHAT_BASE_URL"); err != nil {
		return nil, err
	}
	if c.ChatAPIKey, err = required("CHAT_API_KEY"); err != nil {
		return nil, err
	}
	c.EmbeddingModel = os.Getenv("EMBEDDING_MODEL")
	if c.EmbeddingModel == "" {
		c.EmbeddingModel = "course-embedding"
	}
	c.ChatModel = os.Getenv("CHAT_MODEL")
	if c.ChatModel == "" {
		c.ChatModel = "course-chat"
	}
	c.QdrantURL = os.Getenv("QDRANT_URL")
	if c.QdrantURL == "" {
		c.QdrantURL = "http://qdrant:6333"
	}
	dim, err := getenvInt64("EMBEDDING_DIM", 2048)
	if err != nil {
		return nil, err
	}
	c.EmbeddingDim = int(dim)
	chunkMax, err := getenvInt64("CHUNK_AUTO_MAX_CHARS", 800)
	if err != nil {
		return nil, err
	}
	c.ChunkAutoMaxChars = int(chunkMax)
	overlap, err := strconv.Atoi(os.Getenv("CHUNK_AUTO_OVERLAP"))
	if err != nil || overlap < 0 {
		return nil, fmt.Errorf("CHUNK_AUTO_OVERLAP 须为非负整数")
	}
	c.ChunkAutoOverlap = int(overlap)
	if c.ChunkAutoOverlap >= c.ChunkAutoMaxChars {
		return nil, fmt.Errorf("CHUNK_AUTO_OVERLAP 须小于 CHUNK_AUTO_MAX_CHARS")
	}
	cutoff, err := getenvFloat("VECTOR_SCORE_THRESHOLD", 0.35)
	if err != nil {
		return nil, err
	}
	c.VectorScoreCutoff = cutoff
	if cutoff > 1 {
		return nil, fmt.Errorf("VECTOR_SCORE_THRESHOLD 须为0到1")
	}
	rrfK, err := getenvInt64("RRF_K", 60)
	if err != nil {
		return nil, err
	}
	c.RRFK = int(rrfK)
	askTopK, err := getenvInt64("ASK_TOP_K", 4)
	if err != nil {
		return nil, err
	}
	c.AskTopK = int(askTopK)

	return c, nil
}
