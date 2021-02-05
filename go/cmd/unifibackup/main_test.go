package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBackupMeta(t *testing.T) {
	meta, err := parseBackupMeta("./testdata/autobackup_meta.json")
	require.NoError(t, err)
	assert.Contains(t, meta, "autobackup_6.0.43_20210203_1500_1612364400175.unf")
	assert.Contains(t, meta, "autobackup_6.0.43_20210204_1500_1612450800014.unf")
	assert.Equal(t,
		&backupMeta{
			Version:  "6.0.43",
			Time:     1612364400175,
			DateTime: time.Date(2021, 2, 3, 15, 0, 0, 0, time.UTC),
			Format:   "bson",
			Days:     0,
			Size:     11536,
		},
		meta["autobackup_6.0.43_20210203_1500_1612364400175.unf"],
	)
	assert.Equal(t,
		&backupMeta{
			Version:  "6.0.43",
			Time:     1612450800014,
			DateTime: time.Date(2021, 2, 4, 15, 0, 0, 0, time.UTC),
			Format:   "bson",
			Days:     0,
			Size:     11232,
		},
		meta["autobackup_6.0.43_20210204_1500_1612450800014.unf"],
	)
}

func TestSelectLatestBackup(t *testing.T) {
	meta, err := parseBackupMeta("./testdata/autobackup_meta.json")
	require.NoError(t, err)

	assert.Equal(t, "autobackup_6.0.43_20210204_1500_1612450800014.unf", selectLatestBackup(meta))
}
