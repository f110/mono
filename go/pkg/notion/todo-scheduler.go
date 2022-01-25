package notion

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"go.f110.dev/notion-api/v3"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"

	"go.f110.dev/mono/go/pkg/logger"
)

var maxProcessingTime = 1 * time.Minute

type todoSchedulerConfig struct {
	DatabaseID     string `yaml:"database_id"`
	ScheduleColumn string `yaml:"schedule_column"`
}

type ToDoScheduler struct {
	cron   *cron.Cron
	conf   []*todoSchedulerConfig
	client *notion.Client
}

func NewToDoScheduler(configPath, token string) (*ToDoScheduler, error) {
	f, err := os.Open(configPath)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	var conf []*todoSchedulerConfig
	if err := yaml.NewDecoder(f).Decode(&conf); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	client, err := notion.New(tc, notion.BaseURL)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return &ToDoScheduler{client: client, conf: conf}, nil
}

func (s *ToDoScheduler) Execute(dryRun bool) error {
	return s.run(dryRun)
}

func (s *ToDoScheduler) run(dryRun bool) error {
	for _, config := range s.conf {
		pages, err := s.client.GetPages(
			context.TODO(),
			config.DatabaseID,
			&notion.Filter{
				Property: config.ScheduleColumn,
				Text: &notion.TextFilter{
					IsNotEmpty: true,
				},
			},
			nil,
		)
		if err != nil {
			logger.Log.Info("Failed to get pages", zap.Error(err))
			return err
		}

		pageMap := make(map[string]*notion.Page)
		var schedules []*scheduleEvent
		for _, page := range pages {
			v, ok := page.Properties[config.ScheduleColumn]
			if !ok {
				continue
			}
			if strings.HasPrefix(v.RichText[0].PlainText, "Made by") {
				continue
			}
			e, err := s.parseSchedule(v.RichText[0].PlainText)
			if err != nil {
				logger.Log.Warn("Failed parse schedule spec", zap.Error(err))
				continue
			}
			e.ID = page.ID
			e.Title = page.Properties["Name"].Title[0].PlainText
			schedules = append(schedules, e)
			logger.Log.Debug("Found schedule event", zap.Int("interval", int(e.Interval)), zap.String("id", e.ID))
			pageMap[page.ID] = page
		}

		for _, spec := range schedules {
			switch spec.Interval {
			case intervalWeekly:
				if time.Now().Weekday() != spec.Weekday {
					continue
				}
			case intervalMonthly:
				if time.Now().Day() != spec.Day {
					logger.Log.Debug("Skip because the day is mismatch",
						zap.Int("now", time.Now().Day()),
						zap.Int("spec", spec.Day),
						zap.String("id", spec.ID),
					)
					continue
				}
			}

			lastPage := s.findLastPage(pages, config.ScheduleColumn, spec)
			var shouldCreateNewPage bool
			if lastPage != nil {
				logger.Log.Debug("Found last page", zap.String("id", lastPage.ID), zap.String("spec_id", spec.ID))
				var interval time.Duration
				switch spec.Interval {
				case intervalDaily:
					interval = 24*time.Hour - maxProcessingTime
				case intervalWeekly:
					interval = 24 * time.Hour
				case intervalMonthly:
					interval = 7 * 24 * time.Hour
				}

				logger.Log.Debug("Last page created at", zap.Time("created", lastPage.CreatedTime.Time), zap.Duration("interval", interval))
				if time.Now().After(lastPage.CreatedTime.Add(interval)) {
					shouldCreateNewPage = true
				}
			} else {
				shouldCreateNewPage = true
			}
			if !shouldCreateNewPage {
				continue
			}

			newPage := pageMap[spec.ID].New()
			newPage.Properties[config.ScheduleColumn] = &notion.PropertyData{
				Type: "rich_text",
				RichText: []*notion.RichTextObject{
					{
						Type: "text",
						Text: &notion.Text{Content: "Made by "},
					},
					{
						Type: "mention",
						Mention: &notion.Mention{
							Type: "page",
							Page: &notion.Meta{ID: spec.ID},
						},
					},
				},
			}
			if dryRun {
				logger.Log.Info("Create page")
			} else {
				_, err = s.client.CreatePage(context.TODO(), newPage)
				if err != nil {
					logger.Log.Warn("Failed to create new page", zap.Error(err))
					return err
				}
			}
		}
	}

	return nil
}

func (s *ToDoScheduler) findLastPage(pages []*notion.Page, col string, schedule *scheduleEvent) *notion.Page {
	var lastPage *notion.Page
	for _, page := range pages {
		if !strings.HasPrefix(page.Properties[col].RichText[0].PlainText, "Made by") {
			continue
		}
		if page.Properties[col].RichText[1].PlainText != schedule.Title {
			continue
		}
		if lastPage == nil {
			lastPage = page
		}
		if page.CreatedTime.After(lastPage.CreatedTime.Time) {
			lastPage = page
		}
	}

	return lastPage
}

type scheduleInterval int

const (
	intervalHourly scheduleInterval = iota + 1
	intervalDaily
	intervalWeekly
	intervalMonthly
)

type scheduleEvent struct {
	ID       string
	Title    string
	Interval scheduleInterval
	Minute   int
	Hour     int
	Day      int
	Weekday  time.Weekday
}

func (s *ToDoScheduler) parseSchedule(schedule string) (*scheduleEvent, error) {
	if strings.HasSuffix(schedule, "of every month") {
		sp := strings.Split(schedule, " ")
		var day int
		switch sp[0] {
		case "1st":
			day = 1
		case "2nd":
			day = 2
		case "3rd":
			day = 3
		case "21st":
			day = 21
		case "22nd":
			day = 22
		case "23rd":
			day = 23
		case "31st":
			day = 31
		default:
			if !strings.HasSuffix(sp[0], "th") {
				return nil, xerrors.New("failed to parse")
			}
			d, err := strconv.ParseInt(strings.TrimSuffix(sp[0], "th"), 10, 32)
			if err != nil {
				return nil, xerrors.Errorf(": %w", err)
			}
			if d > 31 {
				return nil, xerrors.New("the day is out of range")
			}
			day = int(d)
		}
		return &scheduleEvent{Interval: intervalMonthly, Day: day}, nil
	}

	if strings.HasPrefix(schedule, "every") {
		sp := strings.Split(schedule[len("every "):], " ")
		var weekday time.Weekday
		switch sp[0] {
		case "Monday":
			weekday = time.Monday
		case "Tuesday":
			weekday = time.Tuesday
		case "Wednesday":
			weekday = time.Wednesday
		case "Thursday":
			weekday = time.Thursday
		case "Friday":
			weekday = time.Friday
		case "Saturday":
			weekday = time.Saturday
		case "Sunday":
			weekday = time.Sunday
		}
		var minute, hour int
		if len(sp) > 3 {
			if sp[1] == "at" {
				t := strings.Split(sp[2], ":")
				h, err := strconv.ParseInt(t[0], 10, 32)
				if err != nil {
					return nil, xerrors.Errorf(": %w", err)
				}
				m, err := strconv.ParseInt(t[1], 10, 32)
				if err != nil {
					return nil, xerrors.Errorf(": %w", err)
				}
				hour = int(h)
				if sp[3] == "pm" {
					hour += 12
				}
				minute = int(m)
			}
		}

		return &scheduleEvent{Interval: intervalWeekly, Weekday: weekday, Hour: hour, Minute: minute}, nil
	}
	return nil, nil
}

func (s *ToDoScheduler) Start(c string) error {
	s.cron = cron.New()
	_, err := s.cron.AddFunc(c, func() {
		logger.Log.Debug("Schedule check")
		if err := s.run(false); err != nil {
			logger.Log.Warn("Failed to run", zap.Error(err))
		}
	})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	logger.Log.Info("Start cron")
	s.cron.Start()

	return nil
}
