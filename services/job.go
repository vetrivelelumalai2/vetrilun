package services

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Job struct {
	ID        string
	Queue     string
	l         []JobLogItem
	run       func(j *Job)
	observers []chan JobLogItem
	closed    bool
}

type JobLogItemLevel string

const (
	Info           JobLogItemLevel = "info"
	Error          JobLogItemLevel = "error"
	Warn           JobLogItemLevel = "warn"
	Done           JobLogItemLevel = "done"
	InProgress     JobLogItemLevel = "inprogress"
	Finish         JobLogItemLevel = "finish"
	Redirect       JobLogItemLevel = "redirect"
	Download       JobLogItemLevel = "download"
	RenderTemplate JobLogItemLevel = "rendertemplate"
	StatusUpdate   JobLogItemLevel = "statusupdate"
	Close          JobLogItemLevel = "close"
)

var levelMap = map[JobLogItemLevel]log.Level{
	Info:           log.InfoLevel,
	Error:          log.ErrorLevel,
	Warn:           log.WarnLevel,
	Done:           log.InfoLevel,
	InProgress:     log.InfoLevel,
	Finish:         log.InfoLevel,
	Download:       log.InfoLevel,
	Redirect:       log.InfoLevel,
	StatusUpdate:   log.InfoLevel,
	RenderTemplate: log.InfoLevel,
	Close:          log.InfoLevel,
}

type JobLogItem struct {
	Level     JobLogItemLevel `json:"level,omitempty"`
	Message   string          `json:"message,omitempty"`
	Tag       string          `json:"tag,omitempty"`
	Location  string          `json:"location,omitempty"`
	Template  string          `json:"template,omitempty"`
	Body      string          `json:"body,omitempty"`
	Timestamp time.Time       `json:"timestamp,omitempty"`
}

func NewJob(id string, queue string, run func(j *Job)) *Job {
	return &Job{
		ID:        id,
		Queue:     queue,
		run:       run,
		l:         []JobLogItem{},
		observers: []chan JobLogItem{},
	}
}

func (s *Job) Run() {
	s.run(s)
}

func (s *Job) ObserveLog() chan JobLogItem {
	c := make(chan JobLogItem, 100)
	s.observers = append(s.observers, c)
	for _, i := range s.l {
		c <- i
	}
	return c
}

func (s *Job) log(l JobLogItem) {
	l.Timestamp = time.Now()
	s.l = append(s.l, l)
	for _, o := range s.observers {
		o <- l
		if l.Level == Close {
			close(o)
		}
	}
	message := l.Message
	if l.Level == Done {
		message = "done"
	}
	if l.Level == Finish {
		message = "finish"
	}
	if l.Level == Redirect {
		message = "redirect"
	}
	if l.Level == StatusUpdate {
		message = "statusupdate"
	}
	if l.Level == RenderTemplate {
		message = "rendertemplate"
	}
	log.WithFields(log.Fields{
		"ID":       s.ID,
		"Queue":    s.Queue,
		"Tag":      l.Tag,
		"Location": l.Location,
		"Template": l.Template,
		"Body":     l.Body,
	}).Log(levelMap[l.Level], message)
}

func (s *Job) Info(message string) *Job {
	s.log(JobLogItem{
		Level:   Info,
		Message: message,
	})
	return s
}

func (s *Job) Error(message string, tag string) *Job {
	s.log(JobLogItem{
		Level:   Error,
		Message: message,
		Tag:     tag,
	})
	return s
}

func (s *Job) InProgress(message string, tag string) *Job {
	s.log(JobLogItem{
		Level:   InProgress,
		Message: message,
		Tag:     tag,
	})
	return s
}

func (s *Job) StatusUpdate(message string, tag string) *Job {
	s.log(JobLogItem{
		Level:   StatusUpdate,
		Message: message,
		Tag:     tag,
	})
	return s
}

func (s *Job) Done(tag string) *Job {
	s.log(JobLogItem{
		Level: Done,
		Tag:   tag,
	})
	return s
}
func (s *Job) Finish() *Job {
	s.log(JobLogItem{
		Level:   Finish,
		Message: "success!",
	})
	return s
}

func (s *Job) Download(url string) *Job {
	s.log(JobLogItem{
		Level:    Download,
		Location: url,
	})
	return s
}

func (s *Job) Redirect(url string) *Job {
	s.log(JobLogItem{
		Level:    Redirect,
		Location: url,
	})
	return s
}

func (s *Job) RenderTemplate(name string, body string) *Job {
	s.log(JobLogItem{
		Level:    RenderTemplate,
		Template: name,
		Body:     body,
	})
	return s
}

func (s *Job) FinishWithMessage(m string) *Job {
	s.log(JobLogItem{
		Level:   Finish,
		Message: m,
	})
	return s
}

func (s *Job) Close() {
	if s.closed {
		return
	}
	s.closed = true
	s.log(JobLogItem{
		Level: Close,
	})
}

type Jobs struct {
	queue string
	mux   sync.Mutex
	jobs  map[string]*Job
}

func NewJobs(queue string) *Jobs {
	return &Jobs{
		queue: queue,
		jobs:  map[string]*Job{},
	}
}

func (s *Jobs) Enqueue(ctx context.Context, id string, r func(j *Job)) *Job {
	s.mux.Lock()
	defer s.mux.Unlock()
	if _, ok := s.jobs[id]; ok {
		return s.jobs[id]
	}
	j := NewJob(id, s.queue, r)
	s.jobs[id] = j
	go func() {
		j.Run()
		<-ctx.Done()
		j.Close()
		s.mux.Lock()
		defer s.mux.Unlock()
		delete(s.jobs, id)
	}()
	return j
}

func (s *Jobs) Log(id string) chan JobLogItem {
	c := make(chan JobLogItem, 100)
	if _, ok := s.jobs[id]; ok {
		go func() {
			for i := range s.jobs[id].ObserveLog() {
				c <- i
			}
			close(c)
		}()
	} else {
		close(c)
	}
	return c
}

type JobQueues map[string]*Jobs

var queueMux sync.Mutex

func NewJobQueues() *JobQueues {
	return &JobQueues{}
}

func (s JobQueues) Get(name string) *Jobs {
	return s[name]
}

func (s JobQueues) GetOrCreate(name string) *Jobs {
	queueMux.Lock()
	defer queueMux.Unlock()
	_, ok := s[name]
	if !ok {
		s[name] = NewJobs(name)
	}
	return s[name]
}
