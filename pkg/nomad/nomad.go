package nomad

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"time"
)

type Job struct {
	Meta struct {
		Version *string `json:"VERSION"`
	} `json:"Meta"`
	JobModifyIndex int

	other map[string]json.RawMessage
}

type jobRequest struct {
	Job            *Job
	EnforceIndex   bool
	JobModifyIndex int
}

func (j *Job) UnmarshalJSON(bs []byte) error {
	ref := reflect.ValueOf(j).Elem()
	fields := map[string]reflect.Value{}

	for i := 0; i != ref.NumField(); i++ {
		fieldType := ref.Type().Field(i)

		name := strings.Split(fieldType.Tag.Get("json"), ",")[0]
		if name == "" {
			name = fieldType.Name
		}

		fields[name] = ref.Field(i)
	}

	err := json.Unmarshal(bs, &j.other)
	if err != nil {
		return err
	}

	for key, chunk := range j.other {
		if field, found := fields[key]; found {
			err = json.Unmarshal(chunk, field.Addr().Interface())
			if err != nil {
				return err
			}

			delete(j.other, key)
		}
	}

	return nil
}

func (j Job) MarshalJSON() ([]byte, error) {
	fields := make(map[string]json.RawMessage)

	for k, v := range j.other {
		fields[k] = v
	}

	ref := reflect.ValueOf(&j).Elem()

	for i := 0; i != ref.NumField(); i++ {
		fieldType := ref.Type().Field(i)
		field := ref.Field(i)

		name := strings.Split(fieldType.Tag.Get("json"), ",")[0]
		if name == "" {
			name = fieldType.Name
		}

		if field.CanInterface() {
			value, err := json.Marshal(field.Interface())
			if err != nil {
				return nil, err
			}

			fields[name] = json.RawMessage(value)
		}
	}

	return json.Marshal(fields)
}

func (j *Job) IsUpgradeable() bool {
	return j.Meta.Version != nil
}

type Client struct {
	baseUrl    string
	httpClient *http.Client
}

type ClientOption func(c *Client)

func WithHttpClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

func NewClient(baseUrl string, opts ...ClientOption) *Client {
	c := &Client{
		baseUrl: baseUrl,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.httpClient == nil {
		c.httpClient = &http.Client{
			Timeout: 5 * time.Second,
		}
	}

	return c
}

func (c *Client) ReadJob(ctx context.Context, jobId string) (*Job, error) {
	req, err := http.NewRequest("GET", c.baseUrl + "/v1/job/" + jobId, nil)
	if err != nil {
		return nil, err
	}
	req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("nomad api bad response code %d while reading job", resp.StatusCode))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var job Job
	err = json.Unmarshal(body, &job)
	if err != nil {
		return nil, err
	}

	return &job, nil
}

func (c *Client) CreateJob(ctx context.Context, jobId string, job *Job) error {
	jobRequest := jobRequest{
		Job: job,
		EnforceIndex: true,
		JobModifyIndex: job.JobModifyIndex,
	}

	b, err := json.Marshal(jobRequest)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(b)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.baseUrl + "/v1/job/" + jobId, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("nomad api bad response code %d while creating job", resp.StatusCode))
	}

	return nil
}
