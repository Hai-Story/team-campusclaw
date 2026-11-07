package grading

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

func LoadSettings() (Settings, error) {
	var c Settings
	for _, key := range []string{"AMQP_URL", "CHAT_BASE_URL", "CHAT_API_KEY", "CHAT_MODEL", "GRADE_TIMEOUT_SECONDS", "GRADE_LEASE_SECONDS", "GRADE_MAX_ATTEMPTS", "GRADE_RETRY_BASE_MS", "OUTBOX_POLL_MS"} {
		if os.Getenv(key) == "" {
			return c, fmt.Errorf("缺少必需环境变量 %s", key)
		}
	}
	c.AMQPURL = os.Getenv("AMQP_URL")
	c.ChatURL = os.Getenv("CHAT_BASE_URL")
	c.ChatKey = os.Getenv("CHAT_API_KEY")
	c.Model = os.Getenv("CHAT_MODEL")
	u, err := url.Parse(c.AMQPURL)
	if err != nil || u.Host == "" || u.User == nil || (u.Scheme != "amqp" && u.Scheme != "amqps") {
		return c, fmt.Errorf("AMQP_URL 格式不合法")
	}
	u, err = url.Parse(c.ChatURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return c, fmt.Errorf("CHAT_BASE_URL 格式不合法")
	}
	items := []struct {
		key      string
		min, max int
		target   *time.Duration
		unit     time.Duration
	}{{"GRADE_TIMEOUT_SECONDS", 1, 600, &c.Timeout, time.Second}, {"GRADE_LEASE_SECONDS", 2, 3600, &c.Lease, time.Second}, {"GRADE_RETRY_BASE_MS", 10, 30000, &c.RetryBase, time.Millisecond}, {"OUTBOX_POLL_MS", 50, 60000, &c.Poll, time.Millisecond}}
	for _, item := range items {
		n, e := strconv.Atoi(os.Getenv(item.key))
		if e != nil || n < item.min || n > item.max {
			return c, fmt.Errorf("%s 超出范围", item.key)
		}
		*item.target = time.Duration(n) * item.unit
	}
	n, err := strconv.Atoi(os.Getenv("GRADE_MAX_ATTEMPTS"))
	if err != nil || n < 1 || n > 10 {
		return c, fmt.Errorf("GRADE_MAX_ATTEMPTS 须为1至10")
	}
	c.MaxAttempts = n
	if c.Lease <= c.Timeout {
		return c, fmt.Errorf("GRADE_LEASE_SECONDS 须大于 GRADE_TIMEOUT_SECONDS")
	}
	return c, nil
}
