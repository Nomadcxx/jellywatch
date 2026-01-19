package compliance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePathComponents_ValidPath(t *testing.T) {
	path := "/tv/Show (1989)/Season 01/file.mkv"
	components, err := ParsePathComponents(path)

	assert.NoError(t, err)
	assert.Equal(t, path, components.FullPath)
	assert.Equal(t, "file.mkv", components.Filename)
	assert.Equal(t, "Season 01", components.SeasonFolder)
	assert.Equal(t, "Show (1989)", components.ShowFolder)
	assert.Equal(t, "/tv", components.LibraryRoot)

	ctx, err := components.GetContext()
	assert.NoError(t, err)
	assert.Equal(t, "Show", ctx.ShowName)
	assert.Equal(t, "1989", ctx.Year)
}

func TestParsePathComponents_EmptyPath(t *testing.T) {
	components, err := ParsePathComponents("")
	assert.Error(t, err)
	assert.Nil(t, components)
	assert.Contains(t, err.Error(), "empty path")
}

func TestParsePathComponents_GetContextWithError(t *testing.T) {
	path := "/invalid/path"
	components, err := ParsePathComponents(path)
	assert.NoError(t, err)
	assert.NotNil(t, components)

	ctx, err := components.GetContext()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path too shallow")
	assert.Equal(t, FolderContext{}, ctx)
}

func BenchmarkParsePathComponents(b *testing.B) {
	path := "/tv/The Simpsons (1989)/Season 01/Simpsons S01E01.mkv"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParsePathComponents(path)
	}
}

func BenchmarkExtractFolderContext(b *testing.B) {
	path := "/tv/The Simpsons (1989)/Season 01/Simpsons S01E01.mkv"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExtractFolderContext(path)
	}
}
