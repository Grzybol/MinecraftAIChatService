package logging

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	elasticLogChannelSize = 512
	elasticRequestTimeout = 5 * time.Second
)

var elasticDiagLogger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.LUTC)

type ElasticLogger struct {
	client   *http.Client
	endpoint string
	apiKey   string
	queue    chan logEntry
	stop     chan struct{}
	wg       sync.WaitGroup
}

type logEntry struct {
	Timestamp time.Time              `json:"@timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"logmessage"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

func NewElasticLogger(url, index, apiKey string, verifyCert bool) (*ElasticLogger, error) {
	url = strings.TrimSpace(url)
	index = strings.Trim(strings.TrimSpace(index), "/")
	if url == "" || index == "" {
		return nil, errors.New("elastic url and index must be set")
	}
	endpoint := strings.TrimRight(url, "/") + "/" + index + "/_doc"
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if !verifyCert {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = true
	}
	logger := &ElasticLogger{
		client: &http.Client{
			Timeout:   elasticRequestTimeout,
			Transport: transport,
		},
		endpoint: endpoint,
		apiKey:   strings.TrimSpace(apiKey),
		queue:    make(chan logEntry, elasticLogChannelSize),
		stop:     make(chan struct{}),
	}
	logElasticInfo("elastic_logger_initialized endpoint=%s verify_cert=%t api_key_set=%t", endpoint, verifyCert, strings.TrimSpace(apiKey) != "")
	logger.wg.Add(1)
	go logger.run()
	return logger, nil
}

func (l *ElasticLogger) Close() error {
	close(l.stop)
	l.wg.Wait()
	return nil
}

func (l *ElasticLogger) Enqueue(entry logEntry) {
	select {
	case l.queue <- entry:
	default:
	}
}

func (l *ElasticLogger) run() {
	defer l.wg.Done()
	for {
		select {
		case entry := <-l.queue:
			l.send(entry)
		case <-l.stop:
			for {
				select {
				case entry := <-l.queue:
					l.send(entry)
				default:
					return
				}
			}
		}
	}
}

func (l *ElasticLogger) send(entry logEntry) {
	payload := map[string]interface{}{
		"@timestamp":  entry.Timestamp.UTC().Format(time.RFC3339Nano),
		"level":       entry.Level,
		"logmessage":  entry.Message,
		"transaction": entry.Fields["transaction_id"],
	}
	for key, value := range entry.Fields {
		payload[key] = value
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	logElasticInfo("elastic_send_attempt endpoint=%s payload_bytes=%d", l.endpoint, len(body))
	req, err := http.NewRequest(http.MethodPost, l.endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if l.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("ApiKey %s", l.apiKey))
	}
	resp, err := l.client.Do(req)
	if err != nil {
		logElasticInfo("elastic_send_failed endpoint=%s error=%v", l.endpoint, err)
		return
	}
	logElasticInfo("elastic_send_response status=%s", resp.Status)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		bodyPreview, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		logElasticInfo("elastic_send_non_2xx status=%s body=%q", resp.Status, strings.TrimSpace(string(bodyPreview)))
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

func logElasticInfo(format string, args ...any) {
	elasticDiagLogger.Printf("[INFO] "+format, args...)
}
