package control

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCancelSignalRoundTrip(t *testing.T) {
	sig := CancelSignal{JobID: uuid.New(), RequestedBy: "tester"}
	data, err := sig.Marshal()
	require.NoError(t, err)

	decoded, err := UnmarshalCancelSignal(data)
	require.NoError(t, err)
	require.Equal(t, sig.JobID, decoded.JobID)
	require.Equal(t, "tester", decoded.RequestedBy)
	require.False(t, decoded.RequestedAt.IsZero())
}

func TestUnmarshalCancelSignalMissingJobID(t *testing.T) {
	_, err := UnmarshalCancelSignal([]byte(`{"requested_by":"tester"}`))
	require.Error(t, err)
}
