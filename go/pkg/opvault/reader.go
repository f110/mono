package opvault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/xerrors"
)

var (
	ErrInvalidFormat = xerrors.New("opvault: Invalid format")
	ErrInvalidData   = xerrors.New("opvault: Invalid data")
	ErrLocked        = xerrors.New("opvault: Locked")
)

var (
	profilePrefix    = []byte("var profile=")
	bandPrefix       = []byte("ld(")
	folderPrefix     = []byte("loadFolders(")
	opdataPrefix     = []byte("opdata01")
	attachmentPrefix = []byte("OPCLDAT")
)

type Reader struct {
	Profile *Profile

	dir     string
	err     error
	folders map[string]*Folder
	items   map[string]*Item
}

func NewReader(dir string) *Reader {
	return &Reader{dir: dir}
}

func (r *Reader) Err() error {
	return r.err
}

func (r *Reader) NextItem() bool {
	if err := r.ensureReadProfile(); err != nil {
		return false
	}

	return true
}

func (r *Reader) Items() map[string]*Item {
	if err := r.ensureReadProfile(); err != nil {
		r.err = xerrors.Errorf(": %w", err)
		return nil
	}
	if err := r.Profile.Decrypt(); err != nil {
		r.err = xerrors.Errorf(": %w", err)
		return nil
	}
	if err := r.readItems(); err != nil {
		r.err = xerrors.Errorf(": %w", err)
		return nil
	}

	return r.items
}

func (r *Reader) Folders() map[string]*Folder {
	if err := r.ensureReadProfile(); err != nil {
		r.err = xerrors.Errorf(": %w", err)
		return nil
	}
	if err := r.Profile.Decrypt(); err != nil {
		r.err = xerrors.Errorf(": %w", err)
		return nil
	}
	if err := r.readFolders(); err != nil {
		r.err = xerrors.Errorf(": %w", err)
		return nil
	}

	return r.folders
}

func (r *Reader) Unlock(password string) error {
	if err := r.ensureReadProfile(); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if err := r.Profile.Unlock(password); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (r *Reader) ensureReadProfile() error {
	if r.Profile == nil {
		if err := r.readProfile(); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reader) readProfile() error {
	buf, err := ioutil.ReadFile(filepath.Join(r.dir, "default", "profile.js"))
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if !bytes.HasPrefix(buf, profilePrefix) {
		return xerrors.Errorf(": %w", ErrInvalidFormat)
	}

	// Trim prefix and suffix to valid JSON
	buf = buf[len(profilePrefix):]
	buf = buf[:len(buf)-1]
	p := &Profile{}
	if err := json.Unmarshal(buf, p); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	p.CreatedAt = time.Unix(p.CreatedAtUnix, 0)
	p.UpdatedAt = time.Unix(p.UpdatedAtUnix, 0)

	r.Profile = p
	return nil
}

func (r *Reader) readItems() error {
	bandFiles, err := filepath.Glob(filepath.Join(r.dir, "default", "band_*.js"))
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	attachments, err := filepath.Glob(filepath.Join(r.dir, "default", "*.attachment"))
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	attachmentItemUUID := make(map[string][]string)
	for _, v := range attachments {
		b := filepath.Base(v)
		s := strings.SplitN(b, "_", 2)
		if len(s) != 2 {
			continue
		}
		if _, ok := attachmentItemUUID[s[0]]; !ok {
			attachmentItemUUID[s[0]] = make([]string, 0)
		}

		attachmentItemUUID[s[0]] = append(attachmentItemUUID[s[0]], v)
	}

	items := make(map[string]*Item)
	for _, bandFile := range bandFiles {
		item, err := r.readBand(bandFile)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		for k, v := range item {
			if files, ok := attachmentItemUUID[k]; ok {
				attachments, err := r.readAttachments(v, files)
				if err != nil {
					return xerrors.Errorf(": %w", err)
				}
				v.HasAttachment = true
				v.Attachments = attachments
			}

			items[k] = v
		}
	}
	r.items = items

	return nil
}

func (r *Reader) readBand(file string) (map[string]*Item, error) {
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	if !bytes.HasPrefix(buf, bandPrefix) {
		return nil, xerrors.Errorf(": %w", ErrInvalidFormat)
	}

	// Trim prefix and suffix to valid JSON
	buf = buf[len(bandPrefix):]
	buf = buf[:len(buf)-2]
	items := make(map[string]*Item)
	if err := json.Unmarshal(buf, &items); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	for k, v := range items {
		v.Created = time.Unix(v.CreatedUnix, 0)
		v.Updated = time.Unix(v.UpdatedUnix, 0)

		if err := v.Decrypt(
			r.Profile.MasterHMACKey,
			r.Profile.MasterEncryptionKey,
			r.Profile.OverviewHMACKey,
			r.Profile.OverviewEncryptionKey,
		); err != nil {
			return nil, xerrors.Errorf("opvault: failed decrypt item %s: %w", k, err)
		}
	}

	return items, nil
}

func (r *Reader) readAttachments(item *Item, files []string) ([]*Attachment, error) {
	attachments := make([]*Attachment, 0)
	for _, v := range files {
		attachment, err := r.readAttachment(item, v)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		attachments = append(attachments, attachment)
	}

	return attachments, nil
}

func (r *Reader) readAttachment(item *Item, file string) (*Attachment, error) {
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	if !bytes.HasPrefix(buf, attachmentPrefix) {
		return nil, xerrors.Errorf(": %w", ErrInvalidFormat)
	}
	metadataSize := binary.LittleEndian.Uint16(buf[8:10])
	metadataBuf := buf[16 : 16+metadataSize]
	metadata := &AttachmentMetadata{}
	if err := json.Unmarshal(metadataBuf, metadata); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	decryptedOverview, err := decryptOpdata(metadata.OverviewRaw, r.Profile.OverviewHMACKey, r.Profile.OverviewEncryptionKey)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	metadata.Overview = string(decryptedOverview)
	metadata.CreatedAt = time.Unix(metadata.CreatedAtUnix, 0)
	metadata.UpdatedAt = time.Unix(metadata.UpdatedAtUnix, 0)

	encryptionKey, hmacKey, _ := item.decryptKey(nil, nil)
	return &Attachment{
		OriginalFile:  file,
		Version:       int8(buf[7]),
		Metadata:      metadata,
		encryptionKey: encryptionKey,
		hmacKey:       hmacKey,
	}, nil
}

func (r *Reader) readFolders() error {
	buf, err := ioutil.ReadFile(filepath.Join(r.dir, "default", "folders.js"))
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if !bytes.HasPrefix(buf, folderPrefix) {
		return xerrors.Errorf(": %w", ErrInvalidFormat)
	}

	// Trim prefix and suffix to valid JSON
	buf = buf[len(folderPrefix):]
	buf = buf[:len(buf)-2]
	folders := make(map[string]*Folder)
	if err := json.Unmarshal(buf, &folders); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	for k, v := range folders {
		if err := v.Decrypt(r.Profile.OverviewHMACKey, r.Profile.OverviewEncryptionKey); err != nil {
			return xerrors.Errorf("opvault: Failed decrypt folder %s: %w", k, err)
		}
	}

	r.folders = folders
	return nil
}

type Profile struct {
	UUID           string `json:"uuid"`
	LastUpdatedBy  string `json:"lastUpdatedBy"`
	ProfileName    string `json:"profileName"`
	SaltRaw        string `json:"salt"`
	PasswordHint   string `json:"passwordHint"`
	MasterKeyRaw   string `json:"masterKey"`
	Iterations     int    `json:"iterations"`
	OverviewKeyRaw string `json:"overviewKey"`
	CreatedAtUnix  int64  `json:"createdAt"`
	UpdatedAtUnix  int64  `json:"updatedAt"`

	Salt                  []byte    `json:"-"`
	CreatedAt             time.Time `json:"-"`
	UpdatedAt             time.Time `json:"-"`
	MasterEncryptionKey   []byte    `json:"-"`
	MasterHMACKey         []byte    `json:"-"`
	OverviewEncryptionKey []byte    `json:"-"`
	OverviewHMACKey       []byte    `json:"-"`

	encryptionKey []byte
	hmacKey       []byte
}

type Item struct {
	UUID                 string   `json:"uuid"`
	Category             Category `json:"category"`
	DetailRaw            string   `json:"d"`
	Favorite             int      `json:"fave"`
	Folder               string   `json:"folder"`
	HMAC                 string   `json:"hmac"`
	Key                  string   `json:"k"`
	OverviewRaw          string   `json:"o"`
	Trashed              bool     `json:"trashed"`
	TransactionTimestamp int64    `json:"tx"`
	CreatedUnix          int64    `json:"created"`
	UpdatedUnix          int64    `json:"updated"`

	Detail        string        `json:"-"`
	Overview      string        `json:"-"`
	HasAttachment bool          `json:"-"`
	Attachments   []*Attachment `json:"-"`
	Created       time.Time     `json:"-"`
	Updated       time.Time     `json:"-"`

	encryptionKey []byte
	hmacKey       []byte
}

type Attachment struct {
	OriginalFile string
	Version      int8
	Metadata     *AttachmentMetadata

	encryptionKey []byte
	hmacKey       []byte
	data          []byte
}

type AttachmentMetadata struct {
	UUID                 string `json:"UUID"`
	ItemUUID             string `json:"itemUUID"`
	ContentsSize         int    `json:"contentsSize"`
	External             bool   `json:"external"`
	TransactionTimestamp int64  `json:"txTimestamp"`
	OverviewRaw          string `json:"overview"`
	CreatedAtUnix        int64  `json:"createdAt"`
	UpdatedAtUnix        int64  `json:"updatedAt"`

	Overview  string    `json:"-"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

type Folder struct {
	UUID                 string `json:"uuid"`
	OverviewRaw          string `json:"overview"`
	TransactionTimestamp int64  `json:"tx"`
	Smart                bool   `json:"smart"`
	UpdatedUnix          int64  `json:"updated"`
	CreatedUnix          int64  `json:"created"`

	Overview string    `json:"-"`
	Created  time.Time `json:"-"`
	Updated  time.Time `json:"-"`
}

func (p *Profile) Unlock(password string) error {
	s, err := base64.StdEncoding.DecodeString(p.SaltRaw)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	p.Salt = s

	key := pbkdf2.Key([]byte(password), p.Salt, p.Iterations, 64, sha512.New)
	p.encryptionKey = key[:32]
	p.hmacKey = key[32:]

	return nil
}

func (p *Profile) Decrypt() error {
	if p.hmacKey == nil || p.encryptionKey == nil {
		return xerrors.Errorf(": %w", ErrLocked)
	}
	if p.OverviewEncryptionKey != nil && p.MasterEncryptionKey != nil {
		return nil
	}

	decryptedOverviewKey, err := decryptOpdata(p.OverviewKeyRaw, p.hmacKey, p.encryptionKey)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	h := sha512.New()
	h.Write(decryptedOverviewKey)
	keys := h.Sum(nil)
	p.OverviewEncryptionKey = keys[:32]
	p.OverviewHMACKey = keys[32:]

	decryptedMasterKey, err := decryptOpdata(p.MasterKeyRaw, p.hmacKey, p.encryptionKey)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	h = sha512.New()
	h.Write(decryptedMasterKey)
	keys = h.Sum(nil)
	p.MasterEncryptionKey = keys[:32]
	p.MasterHMACKey = keys[32:]

	return nil
}

func (i *Item) Decrypt(masterHMACKey, masterEncryptionKey, overviewHMACKey, overviewEncryptionKey []byte) error {
	itemEncryptionKey, itemHMACKey, err := i.decryptKey(masterHMACKey, masterEncryptionKey)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	decryptedDetail, err := decryptOpdata(i.DetailRaw, itemHMACKey, itemEncryptionKey)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	i.Detail = string(decryptedDetail)

	decryptedOverview, err := decryptOpdata(i.OverviewRaw, overviewHMACKey, overviewEncryptionKey)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	i.Overview = string(decryptedOverview)

	return nil
}

func (i *Item) decryptKey(hmacKey, encryptionKey []byte) (itemEncryptionKey []byte, itemHMACKey []byte, err error) {
	if i.encryptionKey != nil && i.hmacKey != nil {
		return i.encryptionKey, i.hmacKey, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(i.Key)
	if err != nil {
		return nil, nil, xerrors.Errorf(": %w", err)
	}
	data := decoded[:len(decoded)-32]
	mac := decoded[len(decoded)-32:]

	h := hmac.New(sha256.New, hmacKey)
	if _, err := h.Write(data); err != nil {
		return nil, nil, xerrors.Errorf(": %w", err)
	}
	calced := h.Sum(nil)
	if !bytes.Equal(mac, calced) {
		return nil, nil, xerrors.Errorf(": %w", ErrInvalidData)
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, nil, xerrors.Errorf(": %w", err)
	}
	cbc := cipher.NewCBCDecrypter(block, data[:16])
	keys := make([]byte, 64)
	copy(keys, data[16:])
	cbc.CryptBlocks(keys, keys)

	i.encryptionKey = keys[:32]
	i.hmacKey = keys[32:]
	return i.encryptionKey, i.hmacKey, nil
}

func (a *Attachment) Data() ([]byte, error) {
	if a.data != nil {
		return a.data, nil
	}

	buf, err := ioutil.ReadFile(a.OriginalFile)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	metadataSize := binary.LittleEndian.Uint16(buf[8:10])
	iconSize := binary.LittleEndian.Uint32(buf[12:16])
	data, err := decryptData(buf[16+int(metadataSize)+int(iconSize):], a.hmacKey, a.encryptionKey)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return data, nil
}

func (f *Folder) Decrypt(hmacKey, encryptionKey []byte) error {
	data, err := decryptOpdata(f.OverviewRaw, hmacKey, encryptionKey)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	f.Overview = string(data)

	return nil
}

func decryptOpdata(raw string, hmacKey, encryptionKey []byte) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	if !bytes.HasPrefix(decoded, opdataPrefix) {
		return nil, xerrors.Errorf(": %w", ErrInvalidFormat)
	}

	return decryptData(decoded, hmacKey, encryptionKey)
}

func decryptData(decoded, hmacKey, encryptionKey []byte) ([]byte, error) {
	data := decoded[:len(decoded)-32]
	mac := decoded[len(decoded)-32:]

	h := hmac.New(sha256.New, hmacKey)
	if _, err := h.Write(data); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	calced := h.Sum(nil)
	if !bytes.Equal(mac, calced) {
		return nil, xerrors.Errorf(": %w", ErrInvalidData)
	}

	length := binary.LittleEndian.Uint64(decoded[8:16])
	iv := decoded[16:32]

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	cbc := cipher.NewCBCDecrypter(block, iv)
	cbc.CryptBlocks(data, data)

	return data[len(data)-int(length):], nil
}
