package schema

import (
	"testing"

	"github.com/dadav/helm-schema/pkg/chart"
	"github.com/stretchr/testify/assert"
)

func TestTopoSort(t *testing.T) {
	tests := []struct {
		name          string
		results       []*Result
		allowCircular bool
		want          []string // expected order of chart names
		wantErr       bool
		errorType     error
	}{
		{
			name: "simple dependency chain",
			results: []*Result{
				{Chart: &chart.ChartFile{Name: "A", Dependencies: []*chart.Dependency{{Name: "B"}}}},
				{Chart: &chart.ChartFile{Name: "B", Dependencies: []*chart.Dependency{{Name: "C"}}}},
				{Chart: &chart.ChartFile{Name: "C", Dependencies: []*chart.Dependency{}}},
			},
			allowCircular: false,
			want:          []string{"C", "B", "A"},
			wantErr:       false,
		},
		{
			name: "multiple dependencies",
			results: []*Result{
				{Chart: &chart.ChartFile{Name: "A", Dependencies: []*chart.Dependency{{Name: "B"}, {Name: "C"}}}},
				{Chart: &chart.ChartFile{Name: "B", Dependencies: []*chart.Dependency{{Name: "D"}}}},
				{Chart: &chart.ChartFile{Name: "C", Dependencies: []*chart.Dependency{{Name: "D"}}}},
				{Chart: &chart.ChartFile{Name: "D", Dependencies: []*chart.Dependency{}}},
			},
			allowCircular: false,
			want:          []string{"D", "B", "C", "A"},
			wantErr:       false,
		},
		{
			name: "circular dependency",
			results: []*Result{
				{Chart: &chart.ChartFile{Name: "A", Dependencies: []*chart.Dependency{{Name: "B"}}}},
				{Chart: &chart.ChartFile{Name: "B", Dependencies: []*chart.Dependency{{Name: "A"}}}},
			},
			allowCircular: false,
			want:          nil,
			wantErr:       true,
			errorType:     &CircularError{},
		},
		{
			name: "nil chart in results",
			results: []*Result{
				{Chart: &chart.ChartFile{Name: "A", Dependencies: []*chart.Dependency{{Name: "B"}}}},
				{Chart: nil},
				{Chart: &chart.ChartFile{Name: "B", Dependencies: []*chart.Dependency{}}},
			},
			allowCircular: false,
			want:          []string{"B", "A"},
			wantErr:       false,
		},
		{
			name: "circular dependency allowed",
			results: []*Result{
				{Chart: &chart.ChartFile{Name: "A", Dependencies: []*chart.Dependency{{Name: "B"}}}},
				{Chart: &chart.ChartFile{Name: "B", Dependencies: []*chart.Dependency{{Name: "A"}}}},
			},
			allowCircular: true,
			want:          []string{"A", "B"},
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TopoSort(tt.results, tt.allowCircular)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorType != nil {
					assert.IsType(t, tt.errorType, err)
				}

				// When allowCircular is true and we get a CircularError,
				// we should still get unsorted results back
				if tt.allowCircular && tt.want != nil {
					// Convert results to slice of chart names for easier comparison
					var gotNames []string
					for _, r := range got {
						gotNames = append(gotNames, r.Chart.Name)
					}
					assert.Equal(t, tt.want, gotNames)
				}
				return
			}

			assert.NoError(t, err)

			// Convert results to slice of chart names for easier comparison
			var gotNames []string
			for _, r := range got {
				gotNames = append(gotNames, r.Chart.Name)
			}

			assert.Equal(t, tt.want, gotNames)
		})
	}
}
