package ioutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFanout(t *testing.T) {
	buf := make([]byte, 100)

	f := NewFanout()
	go func() {
		f.Write([]byte("hello"))
	}()

	r1 := f.Tee()
	{
		n, err := r1.Read(buf)
		require.NoError(t, err)
		assert.Equal(t, buf[:n], []byte("hello"))
	}

	r2 := f.Tee()

	go func() {
		f.Write([]byte("world"))
	}()

	{
		n, err := r1.Read(buf)
		require.NoError(t, err)
		assert.Equal(t, buf[:n], []byte("world"))
	}
	{
		n, err := r2.Read(buf)
		require.NoError(t, err)
		assert.Equal(t, buf[:n], []byte("world"))
	}
}
