package repository

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

type publisherBackupReader struct {
	file     *os.File
	info     os.FileInfo
	filename string
	approval string
	entries  map[string]*zip.File
	manifest *publisherBackupManifest
	records  map[string]publisherRecord
}

func backupPublisherName(name string) bool {
	if len(name) == 0 || len(name) > 400 || path.Clean(name) != name || !filepath.IsLocal(filepath.FromSlash(name)) {
		return false
	}
	if name == "private/state.json" || name == "public/current.json" {
		return true
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasSuffix(part, ".") {
			return false
		}
	}
	if strings.HasPrefix(name, "public/metadata/") {
		return delivery.ValidMetadataName(strings.TrimPrefix(name, "public/metadata/"))
	}
	if strings.HasPrefix(name, "public/targets/") {
		return publisherfiles.RelativeJSON(strings.TrimPrefix(name, "public/"))
	}
	if strings.HasPrefix(name, "public/releases/") {
		parts := strings.SplitN(strings.TrimPrefix(name, "public/releases/"), "/", 2)
		return len(parts) == 2 && canonicalHash(parts[0]) && publisherfiles.RelativeJSON(parts[1]) && (strings.HasPrefix(parts[1], "metadata/") || strings.HasPrefix(parts[1], "targets/") || strings.HasPrefix(parts[1], "clients/") || strings.HasPrefix(parts[1], "candidates/"))
	}
	return false
}

// Bound central-directory allocation before archive/zip parses it. This is the
// strict Store/no-comment/non-ZIP64 format produced by our bounded backup writer,
// not a general ZIP extractor. Layout constants follow Go archive/zip sources.
func boundedBackupDirectory(ctx context.Context, f *os.File, size int64) (int, int64, error) {
	var end [22]byte
	if size < 22 || size > maxPublisherBackupOutput {
		return 0, 0, delivery.ErrBudget
	}
	if _, err := f.ReadAt(end[:], size-22); err != nil {
		return 0, 0, err
	}
	u16 := binary.LittleEndian.Uint16
	u32 := binary.LittleEndian.Uint32
	count := int(u16(end[10:12]))
	offset, length := int64(u32(end[16:20])), int64(u32(end[12:16]))
	if u32(end[:4]) != 0x06054b50 || u16(end[4:6]) != 0 || u16(end[6:8]) != 0 || int(u16(end[8:10])) != count || u16(end[20:22]) != 0 || count < 2 || count > 2*maxPublisherPublicFiles+1 || offset < 1 || length < 46 || length > 16<<20 || offset+length != size-22 {
		return 0, 0, errors.New("repository: unsupported or oversized backup directory")
	}
	cursor := offset
	for i := 0; i < count; i++ {
		if err := ctx.Err(); err != nil {
			return 0, 0, err
		}
		var header [46]byte
		if cursor+46 > offset+length {
			return 0, 0, delivery.ErrBudget
		}
		if _, err := f.ReadAt(header[:], cursor); err != nil {
			return 0, 0, err
		}
		name, extra, comment := int64(u16(header[28:30])), int64(u16(header[30:32])), u16(header[32:34])
		if u32(header[:4]) != 0x02014b50 || name < 1 || name > 400 || extra > 128 || comment != 0 || u16(header[34:36]) != 0 || u32(header[20:24]) == 0xffffffff || u32(header[24:28]) == 0xffffffff || u32(header[42:46]) == 0xffffffff {
			return 0, 0, errors.New("repository: unsupported backup entry metadata")
		}
		cursor += 46 + name + extra
	}
	if cursor != offset+length {
		return 0, 0, errors.New("repository: backup directory count differs")
	}
	return count, offset, nil
}

func (b *publisherBackupReader) check(ctx context.Context) error {
	if err := b.identity(ctx); err != nil {
		return err
	}
	hash := sha256.New()
	r := io.NewSectionReader(b.file, 0, b.info.Size())
	buffer := make([]byte, 64<<10)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := r.Read(buffer)
		if n != 0 {
			_, _ = hash.Write(buffer[:n])
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if hex.EncodeToString(hash.Sum(nil)) != b.approval {
		return errors.New("repository: independent backup SHA256 approval changed")
	}
	return b.identity(ctx)
}

func (b *publisherBackupReader) identity(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := publisherPath(b.filename); err != nil {
		return err
	}
	current, err := os.Lstat(b.filename)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(b.info, current) || current.Size() != b.info.Size() {
		return errors.New("repository: approved backup file identity changed")
	}
	after, err := b.file.Stat()
	if err != nil || !os.SameFile(b.info, after) || after.Size() != b.info.Size() {
		return errors.New("repository: backup open identity changed")
	}
	return ctx.Err()
}

func openPublisherBackup(ctx context.Context, filename, approval string) (result *publisherBackupReader, finalErr error) {
	if ctx == nil || !canonicalHash(approval) {
		return nil, errors.New("repository: context and independent whole-backup approval required")
	}
	if err := publisherPath(filename); err != nil {
		return nil, err
	}
	info, err := os.Lstat(filename)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxPublisherBackupOutput || info.Size() < 22 {
		return nil, errors.Join(delivery.ErrBudget, err)
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		if finalErr != nil {
			finalErr = errors.Join(finalErr, f.Close())
		}
	}()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) || info.Size() != opened.Size() {
		return nil, errors.New("repository: backup changed while opening")
	}
	b := &publisherBackupReader{file: f, info: opened, filename: filename, approval: approval, entries: map[string]*zip.File{}}
	if err = b.check(ctx); err != nil {
		return nil, err
	}
	count, central, err := boundedBackupDirectory(ctx, f, info.Size())
	if err != nil {
		return nil, err
	}
	reader, err := zip.NewReader(f, info.Size())
	if err != nil || len(reader.File) != count || reader.Comment != "" {
		return nil, errors.New("repository: invalid bounded backup container")
	}
	type span struct{ from, to int64 }
	var spans []span
	seen := map[string]bool{}
	var total uint64
	for _, entry := range reader.File {
		key := strings.ToLower(entry.Name)
		if seen[key] || entry.Method != zip.Store || entry.Flags & ^uint16(8|0x800) != 0 || !entry.Mode().IsRegular() || entry.Comment != "" || len(entry.Extra) > 128 || entry.UncompressedSize64 == 0 || entry.UncompressedSize64 != entry.CompressedSize64 {
			return nil, errors.New("repository: unsupported/aliased backup object")
		}
		limit := uint64(delivery.MaxEncodedPackageBytes)
		if entry.Name == "backup-manifest.json" {
			limit = maxPublisherBackupManifest
		} else if !backupPublisherName(entry.Name) {
			return nil, errors.New("repository: backup path outside accepted namespaces")
		}
		if entry.UncompressedSize64 > limit || entry.UncompressedSize64 > uint64(maxPublisherBackupData+maxPublisherBackupManifest)-total {
			return nil, delivery.ErrBudget
		}
		total += entry.UncompressedSize64
		offset, err := entry.DataOffset()
		if err != nil || offset < 0 || offset+int64(entry.CompressedSize64) > central {
			return nil, errors.New("repository: backup object crosses container boundary")
		}
		spans = append(spans, span{offset, offset + int64(entry.CompressedSize64)})
		seen[key], b.entries[entry.Name] = true, entry
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].from < spans[j].from })
	for i := 1; i < len(spans); i++ {
		if spans[i].from < spans[i-1].to {
			return nil, errors.New("repository: overlapping backup object bodies")
		}
	}
	if b.entries["backup-manifest.json"] == nil || b.entries["private/state.json"] == nil {
		return nil, errors.New("repository: backup manifest/checkpoint required")
	}
	return b, nil
}

func (b *publisherBackupReader) read(ctx context.Context, name string, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entry := b.entries[name]
	if entry == nil || limit < 1 || entry.UncompressedSize64 > uint64(limit) {
		return nil, errors.New("repository: required bounded backup object missing")
	}
	r, err := entry.Open()
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(r, limit+1))
	if err = errors.Join(readErr, r.Close()); err != nil || int64(len(raw)) > limit || uint64(len(raw)) != entry.UncompressedSize64 {
		return nil, errors.Join(errors.New("repository: backup object body/CRC changed"), err)
	}
	if b.records != nil && name != "backup-manifest.json" {
		record, exists := b.records[name]
		if !exists || int64(len(raw)) != record.Length || digest(raw) != record.SHA256 {
			return nil, errors.New("repository: backup record hash changed")
		}
	}
	return raw, ctx.Err()
}
