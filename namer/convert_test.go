package namer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertToValidName(t *testing.T) {
	t.Run("Test with dot", func(t *testing.T) {
		input := "test.name"
		want := "test_name"
		assert.Equal(t, want, convertToValidName(input, "tmux"))
	})

	t.Run("Test with colon", func(t *testing.T) {
		input := "test:name"
		want := "test_name"
		assert.Equal(t, want, convertToValidName(input, "tmux"))
	})

	t.Run("Test with multiple special characters", func(t *testing.T) {
		input := "test.name:with.multiple"
		want := "test_name_with_multiple"
		assert.Equal(t, want, convertToValidName(input, "tmux"))
	})

	t.Run("Test with single space", func(t *testing.T) {
		input := "my session"
		want := "my_session"
		assert.Equal(t, want, convertToValidName(input, "tmux"))
	})

	t.Run("Test with leading and trailing whitespace", func(t *testing.T) {
		input := "  name  "
		want := "name"
		assert.Equal(t, want, convertToValidName(input, "tmux"))
	})

	t.Run("Test with multiple consecutive spaces", func(t *testing.T) {
		input := "a   b"
		want := "a_b"
		assert.Equal(t, want, convertToValidName(input, "tmux"))
	})

	t.Run("Test with tab character", func(t *testing.T) {
		input := "a\tb"
		want := "a_b"
		assert.Equal(t, want, convertToValidName(input, "tmux"))
	})

	t.Run("Test with whitespace and other special characters", func(t *testing.T) {
		input := "my project.v2:main"
		want := "my_project_v2_main"
		assert.Equal(t, want, convertToValidName(input, "tmux"))
	})

	t.Run("TUIOS escapes separators without aliasing literal names", func(t *testing.T) {
		for input, want := range map[string]string{
			"work/api":   "work%2Fapi",
			`work\api`:   "work%5Capi",
			"work_api":   "work_api",
			"work%2Fapi": "work%252Fapi",
			"work%5Capi": "work%255Capi",
		} {
			assert.Equal(t, want, convertToValidName(input, "tuios"), input)
		}
	})
}
