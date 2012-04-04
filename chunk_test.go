package chunk

import (
	"bytes"
	"io"
	"testing"
)

var testString = []byte{
	'T', 'E', 'S', 'T', 0, 0, 0, 2, 42, 24,
	'F', 'O', 'O', ' ', 0, 0, 0, 20,
	'I', 'N', '1', ' ', 0, 0, 0, 1, 255, 0,
	'I', 'N', '2', ' ', 0, 0, 0, 2, 127, 129,
}

func TestHeader(t *testing.T) {
	f := bytes.NewReader(testString)

	chunk, err := Make(f)
	if err != nil {
		t.Errorf("Failed to create Chunk: '%s'", err)
	}

	if name := chunk.Name(); name != "TEST" {
		t.Errorf("Chunk name '%s' != 'TEST'", name)
	}

	if size := chunk.Size(); size != 2 {
		t.Errorf("Chunk size %d != 2.", size)
	}

	// Try a multi-byte size.
	// NOTE: Invalid file.
	f = bytes.NewReader([]byte{'T', 'E', 'S', 'T', 0x12, 0x34, 0x56, 0x78})
	chunk, _ = Make(f)
	if size := chunk.Size(); size != 305419896 {
		t.Errorf("Multi-byte size %d != 305419896", size)
	}
}

func TestBasicRead(t *testing.T) {
	f := bytes.NewReader(testString)

	chunk, _ := Make(f)

	buffer := make([]byte, 10)
	if n, err := chunk.Read(buffer); n != 2 || err != nil {
		t.Errorf("Failed reading 2 bytes; got %d (%s)", n, err)
	}

	if buffer[0] != 42 || buffer[1] != 24 {
		t.Error("Invalid data in output.")
	}

	// Check that we can't read any more.
	if n, err := chunk.Read(buffer); n > 0 || err != io.EOF {
		t.Errorf("Didn't get EOF when reading past end of buffer (read %d bytes: %s).", n, buffer)
	}
}

func TestSkip(t *testing.T) {
	f := bytes.NewReader(testString)

	chunk, _ := Make(f)
	chunk.Skip()

	// Check that we can't read it.
	buffer := make([]byte, 10)
	if n, err := chunk.Read(buffer); n > 0 || err != io.EOF {
		t.Errorf("Didn't get EOF when reading skipped chunk (read %d bytes: %s).", n, buffer)
	}

	// Check that we've advanced to the *next* chunk.
	if chunkFoo, err := Make(f); err == nil {
		if name := chunkFoo.Name(); name != "FOO " {
			t.Errorf("Next chunk not called \"FOO \" (got \"%s\")", name)
		}
	} else {
		t.Errorf("Error when creating chunk after skipping previous chunk: %s", err)
	}
}

func checkByte(chunk *Chunk, expected byte, t *testing.T) {
	buffer := make([]byte, 1)
	if n, err := chunk.Read(buffer); n != 1 || err != nil {
		t.Errorf("Failed to read one byte (got %d): %s", n, err)
	} else {
		if buffer[0] != expected {
			t.Errorf("Didn't read expected byte (expecting %d, got %d)", expected, buffer[0])
		}
	}
}

func TestSeek(t *testing.T) {
	f := bytes.NewReader(testString)
	chunkTest, _ := Make(f)
	chunkTest.Skip()
	chunkFoo, _ := Make(f)

	// This test will ignore the internal structure of FOO  and simply check that we can seek through it.

	// Relative seek:
	if pos, err := chunkFoo.Seek(2, 1); pos != 2 || err != nil {
		t.Errorf("Failed to seek to position 2 (at %d): %s", pos, err)
	}

	// Read a byte; check it.
	checkByte(chunkFoo, '1', t)

	// Skip relative again.
	if pos, err := chunkFoo.Seek(9, 1); pos != 12 || err != nil {
		t.Errorf("Failed relative seek; at %d; expecting 12 (%s)", pos, err)
	}

	// Check the byte again.
	checkByte(chunkFoo, '2', t)

	// Try relative skipping past the end.
	if _, err := chunkFoo.Seek(20, 1); err == nil {
		t.Error("Seeked past the end; expected EOF")
	}

	// Try an absolute seek
	if pos, err := chunkFoo.Seek(8, 0); pos != 8 || err != nil {
		t.Errorf("Failed to seek to position 8 (at %d): %s", pos, err)
	}

	// Check the byte
	checkByte(chunkFoo, 255, t)

	// Try an absolute seek from the end
	if pos, err := chunkFoo.Seek(-3, 2); pos != 17 || err != nil {
		t.Errorf("Failed to seek to position end-3 (at %d): %s", pos, err)
	}

	// Check a byte
	checkByte(chunkFoo, 2, t)

	// Try a relative seek to exactly the end.
	if pos, err := chunkFoo.Seek(1, 1); pos != 19 || err != nil {
		t.Errorf("Failed relative seek; at %d; expecting 19 (%s)", pos, err)
	}

	// Read the last byte.
	checkByte(chunkFoo, 129, t)
}

// Test if we can read chunks inside chunks
func TestSubChunks(t *testing.T) {
	f := bytes.NewReader(testString)
	chunkTest, _ := Make(f)
	chunkTest.Skip()
	chunkFoo, _ := Make(f)

	chunkIN1, err := Make(chunkFoo)
	if err != nil {
		t.Errorf("Failed to create chunkIN1 from chunkFoo: %s", err)
	}

	if name := chunkIN1.Name(); name != "IN1 " {
		t.Errorf("chunkIN1 name is '%s'; expected 'IN1 '", name)
	}

	chunkIN1.Skip()
	chunkIN2, err := Make(chunkFoo)

	if err != nil {
		t.Errorf("Failed to create chunkIN2 from chunkFoo: %s", err)
	}

	if name := chunkIN2.Name(); name != "IN2 " {
		t.Errorf("chunkIN2 name is '%s'; expected 'IN2 '", name)
	}

	if pos, err := chunkIN2.Seek(1, 1); pos != 1 || err != nil {
		t.Errorf("Failed to seek to position 1 in chunkIN2; at %d: %s", pos, err)
	}

	checkByte(chunkIN2, 129, t)

	// Check that we can't create a chunk when there is no more file.
	_, err = Make(f)
	if err != io.EOF {
		t.Error("Successfully created chunk past EOF")
	}
}

// Test if reading padded chunks allows us to continue to the next chunk
func TestReadPadding(t *testing.T) {
	f := bytes.NewReader(testString)
	chunkTest, _ := Make(f)
	chunkTest.Skip()
	chunkFoo, _ := Make(f)

	chunkIN1, _ := Make(chunkFoo)

	data := make([]byte, chunkIN1.Size())
	chunkIN1.Read(data)
	if !bytes.Equal(data, []byte{255}) {
		t.Error("First chunk didn't have expected content")
	}

	chunkIN2, err := Make(chunkFoo)
	if err != nil {
		t.Errorf("Error creating second chunk: %s", err)
	}
	if name := chunkIN2.Name(); name != "IN2 " {
		t.Errorf("Second chunk named '%s'; expected 'IN2 '", name)
	}
}

func TestTTY(t *testing.T) {
	f := bytes.NewReader(testString)
	chunk, _ := Make(f)
	if chunk.IsTTY() {
		t.Error("Chunk is apparently a TTY!?")
	}
}
