// chunk provides an interface for reading IFF data files.
// The API is based on the Python standard library's "chunk" module.
package chunk

import "io"
import "errors"

// A Chunk represents a single chunk in an IFF file.
// Chunks implement the io.Reader and io.Seeker interfaces.
type Chunk struct {
	file   io.ReadSeeker
	id     string
	size   uint32
	base   uint32
	offset uint32
}

// Returns a new chunk. An instance of Chunk is specifically allowed as the
// argument to New. This is used to read chunks inside other chunks.
func New(f io.ReadSeeker) (*Chunk, error) {
	id := make([]byte, 4)
	if n, err := f.Read(id); n != 4 || err != nil {
		return nil, err
	}
	sizeBits := make([]byte, 4)
	if n, err := f.Read(sizeBits); n != 4 || err != nil {
		return nil, err
	}
	size := (uint32(sizeBits[0]) << 24) | (uint32(sizeBits[1]) << 16) | (uint32(sizeBits[2]) << 8) | uint32(sizeBits[3])
	base, _ := f.Seek(0, 1)
	return &Chunk{f, string(id), size, uint32(base), 0}, nil
}

// Returns the name (ID) of the chunk. This is the first 4 bytes of the chunk.
func (this *Chunk) Name() (id string) {
	return this.id
}

// Returns the size of the chunk.
func (this *Chunk) Size() (size uint32) {
	return this.size
}

// Seek implements the io.Seeker interface.
func (this *Chunk) Seek(offset int64, whence int) (ret int64, err error) {
	switch whence {
	case 1:
		offset += int64(this.offset)
	case 2:
		offset += int64(this.size)
	}
	if offset < 0 || offset > int64(this.size) {
		return int64(this.offset), errors.New("Invalid seek offset")
	}
	pos, err := this.file.Seek(int64(this.base)+offset, 0)
	if err != nil {
		return int64(pos) - int64(this.base), err
	}
	this.offset = uint32(offset)
	return offset, nil
}

// Skips to the end of the chunk. All further reads will return io.EOF.
// If you are not inrested in the contents of the chunk, this method should be called
// so that the file points to the start of the next chunk.
func (this *Chunk) Skip() {
	pos := this.base + this.size
	// We must always start on even boundaries
	if pos&1 == 1 {
		pos++
	}
	this.offset = this.size
	this.file.Seek(int64(pos), 0)
}

// Read implements the io.Reader interface.
func (this *Chunk) Read(buffer []byte) (n int, err error) {
	if this.offset == this.size {
		return 0, io.EOF
	}
	size := uint32(len(buffer))
	if size > this.size-this.offset {
		size = this.size - this.offset
		buffer = buffer[:size]
	}
	n, err = this.file.Read(buffer)
	this.offset += uint32(n)

	// Check if we need to move up one more.
	if this.offset == this.size && (this.size&1) == 1 {
		if _, err := this.file.Seek(1, 1); err == nil {
			this.offset++
		}
	}

	return
}

// Tells you if the chunk is a TTY. Hint: it isn't.
func (this *Chunk) IsTTY() bool {
	return false
}
