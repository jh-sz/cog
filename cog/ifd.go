package cog

type IFD struct {
	NumEntries uint16
	NextOffset uint32

	entriesMap map[uint16]*entry
}

func (ifd *IFD) GetTag(tagID uint16) (*entry, bool) {
	ent, has := ifd.entriesMap[tagID]
	return ent, has
}

// entry has 12 bytes
type entry struct {
	TagID  uint16 // Bytes 0-1
	TypeID uint16 // Bytes 2-3
	Count  uint32 // Bytes 4-7
	value  []byte
}
