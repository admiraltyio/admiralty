package v1

import (
	"testing"

	"github.com/go-test/deep"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
)

func TestPluginDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   runtime.Object
		expected runtime.Object
	}{
		{
			name:   "empty ProxyArgs",
			config: &ProxyArgs{},
			expected: &ProxyArgs{
				FilterWaitDurationSeconds: pointer.Int32(30),
			},
		},
		{
			name: "non-default ProxyArgs",
			config: &ProxyArgs{
				FilterWaitDurationSeconds: pointer.Int32(60),
			},
			expected: &ProxyArgs{
				FilterWaitDurationSeconds: pointer.Int32(60),
			},
		},
		{
			name:   "empty CandidateArgs",
			config: &CandidateArgs{},
			expected: &CandidateArgs{
				PreBindWaitDurationSeconds: pointer.Int32(30),
			},
		},
		{
			name: "non-default CandidateArgs",
			config: &CandidateArgs{
				PreBindWaitDurationSeconds: pointer.Int32(120),
			},
			expected: &CandidateArgs{
				PreBindWaitDurationSeconds: pointer.Int32(120),
			},
		},
	}

	for _, tc := range tests {
		scheme := runtime.NewScheme()
		utilruntime.Must(AddToScheme(scheme))
		t.Run(tc.name, func(t *testing.T) {
			scheme.Default(tc.config)
			diff := deep.Equal(tc.config, tc.expected)
			if len(diff) > 0 {
				t.Errorf("unexpected plugin defaults diff: %v", diff)
			}
		})
	}
}
