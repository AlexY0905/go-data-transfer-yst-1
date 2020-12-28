// Code generated by github.com/whyrusleeping/cbor-gen. DO NOT EDIT.

package datatransfer

import (
	"fmt"
	"io"

	peer "github.com/libp2p/go-libp2p-core/peer"
	cbg "github.com/whyrusleeping/cbor-gen"
	xerrors "golang.org/x/xerrors"
)

var _ = xerrors.Errorf

var lengthBufChannelID = []byte{131}

func (t *ChannelID) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufChannelID); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.Initiator (peer.ID) (string)
	if len(t.Initiator) > cbg.MaxLength {
		return xerrors.Errorf("Value in field t.Initiator was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len(t.Initiator))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string(t.Initiator)); err != nil {
		return err
	}

	// t.Responder (peer.ID) (string)
	if len(t.Responder) > cbg.MaxLength {
		return xerrors.Errorf("Value in field t.Responder was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len(t.Responder))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string(t.Responder)); err != nil {
		return err
	}

	// t.ID (datatransfer.TransferID) (uint64)

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.ID)); err != nil {
		return err
	}

	return nil
}

func (t *ChannelID) UnmarshalCBOR(r io.Reader) error {
	*t = ChannelID{}

	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)

	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 3 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.Initiator (peer.ID) (string)

	{
		sval, err := cbg.ReadStringBuf(br, scratch)
		if err != nil {
			return err
		}

		t.Initiator = peer.ID(sval)
	}
	// t.Responder (peer.ID) (string)

	{
		sval, err := cbg.ReadStringBuf(br, scratch)
		if err != nil {
			return err
		}

		t.Responder = peer.ID(sval)
	}
	// t.ID (datatransfer.TransferID) (uint64)

	{

		maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.ID = TransferID(extra)

	}
	return nil
}
