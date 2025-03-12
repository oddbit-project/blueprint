package clickhouse

import (
	"testing"

	"github.com/oddbit-project/blueprint/db"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name          string
		clientConfig  *ClientConfig
		expectedError error
		expectClient  bool
	}{
		{
			name:          "ValidClientConfig",
			clientConfig:  &ClientConfig{DSN: "clickhouse://default:somePassword@clickhouse:9000/default"},
			expectedError: nil,
			expectClient:  true,
		},
		{
			name:          "InvalidClientConfig",
			clientConfig:  &ClientConfig{DSN: ""},
			expectedError: ErrEmptyDSN,
			expectClient:  false,
		},
	}

	// Run each test case individually
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewClient(tc.clientConfig)

			// Check if error matches expected error
			if tc.expectedError != nil {
				require.NotNil(t, err)
				require.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.Nil(t, err)
			}

			// Check if client is returned when expected
			if tc.expectClient {
				require.NotNil(t, client)
				require.IsType(t, &db.SqlClient{}, client)
				require.Nil(t, client.Connect())
			} else {
				require.Nil(t, client)
			}
		})
	}
}
