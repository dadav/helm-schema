package searching

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArchivesExtractOutsideChartTree(t *testing.T) {
	root := t.TempDir()
	charts := filepath.Join(root, "charts")
	require.NoError(t, os.Mkdir(charts, 0o755))
	file, err := os.Create(filepath.Join(charts, "child.tgz"))
	require.NoError(t, err)
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	content := "apiVersion: v2\nname: child\nversion: 1.0.0\n"
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "child/Chart.yaml", Mode: 0o644, Size: int64(len(content))}))
	_, err = tw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	require.NoError(t, file.Close())
	tempDir, archives, errs := DiscoverArchivesWithSources(root)
	if tempDir != "" {
		t.Cleanup(func() { require.NoError(t, os.RemoveAll(tempDir)) })
	}
	require.Empty(t, errs)
	rel, err := filepath.Rel(root, tempDir)
	require.NoError(t, err)
	require.False(t, filepath.IsLocal(rel), "extraction must not write inside the chart tree")
	discovered, errs := DiscoverChartsWithSources(root, root, "Chart.yaml", nil, archives)
	require.Empty(t, errs)
	require.Len(t, discovered, 1)
	require.Equal(t, "child", discovered[0].Chart.Name)
	require.True(t, discovered[0].Archived)
	require.Equal(t, filepath.Join(charts, "child.tgz", "child", "Chart.yaml"), discovered[0].ID)
	entries, err := os.ReadDir(charts)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}
