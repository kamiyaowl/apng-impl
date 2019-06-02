package apng

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"reflect"
)

type Image struct {
	ihdr Ihdr
	idat Idat
}
type Ihdr struct {
	width     uint32
	height    uint32
	bitDepth  uint8
	colorType uint8
	compress  uint8
	filter    uint8
	interlace uint8
}
type Idat []uint8

func (self *Image) parseIHDR(data []uint8) (err error) {
	if len(data) != 13 {
		return errors.New("IHDRのヘッダサイズは13でなければならない")
	}
	self.ihdr.width = binary.BigEndian.Uint32(data[0:4])
	self.ihdr.height = binary.BigEndian.Uint32(data[4:8])
	self.ihdr.bitDepth = data[8]
	self.ihdr.colorType = data[9]
	self.ihdr.compress = data[10]
	self.ihdr.filter = data[11]
	self.ihdr.interlace = data[12]

	return nil
}
func (self *Image) parseIDAT(data []uint8) (err error) {
	// deflateめんどいしライブラリで許して
	readBuf := bytes.NewBuffer(data)
	zr, err := zlib.NewReader(readBuf)
	if err != nil {
		return err
	}
	defer zr.Close()

	var dst bytes.Buffer
	_, err = dst.ReadFrom(zr)
	if err != nil {
		return err
	}
	if len(self.idat) == 0 {
		self.idat = dst.Bytes()
	} else {
		self.idat = append(self.idat, dst.Bytes()...)
	}
	return nil
}

func (self *Image) Parse(src string) (err error) {
	// read file
	f, err := os.Open(src)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	// png header check
	validSignature := []uint8{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	signature := make([]uint8, len(validSignature))
	n, err := f.Read(signature)
	if n == 0 {
		return errors.New("png headerが読み取れなかった")
	}
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(validSignature, signature) {
		return errors.New("pngファイルではない")
	}
	// read chunks
	isReadIhdr := false
	isReadIdat := false
	isReadIend := false
	for {
		// read header: length(4uint8) + chunk-type
		headersBuf := make([]uint8, 8)
		n, err := f.Read(headersBuf)
		if n < len(headersBuf) {
			// EOF
			break
		}
		if err != nil {
			return errors.New("Chunkヘッダの読み込みエラー")
		}
		length := binary.BigEndian.Uint32(headersBuf[0:4])
		chunkType := string(headersBuf[4:8])
		// read data
		dataBuf := make([]uint8, length)
		n, err = f.Read(dataBuf)
		if err != nil {
			return errors.New("Chunkデータの読み込みエラー")
		}
		// read crc
		crcBuf := make([]uint8, 4)
		n, err = f.Read(crcBuf)
		if err != nil {
			return errors.New("Chunk CRCの読み込みエラー")
		}
		// check crc
		crc := binary.BigEndian.Uint32(crcBuf)
		crcSrc := append(headersBuf[4:8], dataBuf...)
		dataCrc := crc32.ChecksumIEEE(crcSrc)
		if crc != dataCrc {
			fmt.Printf("crc mismatch chunkType:%s crc:%d dataCrc:%d", chunkType, crc, dataCrc)
			continue
		}
		// chunktypeで分岐
		fmt.Printf("chunkType:%s length:%d\n", chunkType, length)
		switch chunkType {
		case "IHDR":
			isReadIhdr = true
			err = self.parseIHDR(dataBuf)
		case "IDAT":
			if !isReadIhdr {
				return errors.New("IHDRが定義される前にIDATが読み出されました")
			}
			isReadIdat = true
			err = self.parseIDAT(dataBuf)
		case "IEND":
			isReadIend = true
			break
		default:
			fmt.Printf("%sは未実装ヘッダです\n", chunkType)
			err = nil
			continue
		}
		// error check
		if err != nil {
			return err
		}
	}
	// 必須チャンクは来ましたか
	if !isReadIhdr {
		return errors.New("IHDRが記述されていません")
	}
	if !isReadIdat {
		return errors.New("IDATが記述されていません")
	}
	if !isReadIend {
		return errors.New("IENDが記述されていません")
	}
	// 完璧やん
	return nil
}
