package jma

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAmedas_GetObservedData(t *testing.T) {
	tr := newMockTransport()

	a := NewAmedas()
	a.client = &http.Client{Transport: tr}
	loc, _ := time.LoadLocation("Asia/Tokyo")
	data, err := a.GetObservedData(context.Background(), Tokyo, 132, time.Date(2024, 9, 23, 0, 35, 0, 0, loc))
	require.NoError(t, err)
	require.Len(t, data, 4)
	assert.Equal(t, &AmedasData{time.Date(2024, 9, 23, 0, 0, 0, 0, time.UTC), 1007.5, 1004.7, 23.7, 88}, data[0])
	assert.Equal(t, &AmedasData{time.Date(2024, 9, 23, 0, 10, 0, 0, time.UTC), 1007.4, 1004.6, 23.7, 87}, data[1])
	assert.Equal(t, &AmedasData{time.Date(2024, 9, 23, 0, 20, 0, 0, time.UTC), 1007.5, 1004.7, 23.6, 88}, data[2])
	assert.Equal(t, &AmedasData{time.Date(2024, 9, 23, 0, 30, 0, 0, time.UTC), 1007.5, 1004.7, 23.6, 88}, data[3])

	data, err = a.GetObservedData(context.Background(), Tokyo, 132, time.Date(2024, 9, 22, 23, 35, 0, 0, loc))
	require.NoError(t, err)
	assert.Len(t, data, 18) // Every 10 minutes for 3 hours
}

func TestAmedas_GetObservedDataRange(t *testing.T) {
	tr := newMockTransport()

	a := NewAmedas()
	a.client = &http.Client{Transport: tr}
	loc, _ := time.LoadLocation("Asia/Tokyo")
	data, err := a.GetObservedDataRange(context.Background(), Tokyo, 132, time.Date(2024, 9, 23, 0, 35, 0, 0, loc), 3*time.Hour)
	require.NoError(t, err)
	require.Len(t, data, 22)
	assert.True(t, sort.SliceIsSorted(data, func(i, j int) bool { return data[i].Time.Before(data[j].Time) }))
}

func TestAmedas_GetObservedDataIter(t *testing.T) {
	tr := newMockTransport()

	a := NewAmedas()
	a.client = &http.Client{Transport: tr}
	loc, _ := time.LoadLocation("Asia/Tokyo")
	dataIter := a.GetObservedDataIter(context.Background(), Tokyo, 132, time.Date(2024, 9, 23, 0, 35, 0, 0, loc))
	var i int
	for v := range dataIter {
		require.NotNil(t, v)
		require.NoError(t, v.Err)
		i++
		if i == 5 {
			break
		}
	}
}

func newMockTransport() http.RoundTripper {
	tr := httpmock.NewMockTransport()
	tr.RegisterRegexpResponder(http.MethodGet,
		regexp.MustCompile(`/bosai/amedas/data/point/\d+/\d+_\d{2}.json$`),
		func(req *http.Request) (*http.Response, error) {
			s := strings.Split(req.URL.Path, "/")
			point, datetime := s[5], s[6]
			datetime = datetime[:strings.Index(datetime, ".")]
			f, err := os.Open(fmt.Sprintf("testdata/%s_%s.json", datetime, point))
			if err != nil {
				return httpmock.NewStringResponse(http.StatusNotFound, ""), nil
			}
			buf, err := io.ReadAll(f)
			if err != nil {
				return httpmock.NewStringResponse(http.StatusInternalServerError, err.Error()), nil
			}
			_ = f.Close()
			return httpmock.NewBytesResponse(http.StatusOK, buf), nil
		},
	)
	return tr
}
