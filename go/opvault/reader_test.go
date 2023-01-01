package opvault

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReader_Folders(t *testing.T) {
	r := NewReader("testdata/onepassword_data")
	err := r.Unlock("freddy")
	require.NoError(t, err)

	folders := r.Folders()
	require.NoError(t, r.Err())
	assert.Equal(t, 3, len(folders))
}

func TestReader_Items(t *testing.T) {
	r := NewReader("testdata/onepassword_data")
	err := r.Unlock("freddy")
	require.NoError(t, err)

	items := r.Items()
	require.NoError(t, r.Err())
	assert.Equal(t, 29, len(items))

	for _, v := range items {
		if v.HasAttachment {
			for _, attach := range v.Attachments {
				data, err := attach.Data()
				require.NoError(t, err)
				assert.Equal(t, len(data), attach.Metadata.ContentsSize)
			}
		}
	}
}
