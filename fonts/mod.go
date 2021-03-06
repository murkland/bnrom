package fonts

import (
	"image"
	"image/color"
	"io"

	"github.com/murkland/bnrom/sprites"
)

type ROMInfo struct {
	TinyOffset int64
	TallOffset int64

	Tall2Offset        int64
	Tall2MetricsOffset int64

	Charmap []rune
}

func tall2Offset(gameTitle string) int64 {
	switch gameTitle {
	case "MEGAMAN6_FXX":
		return 0x006ACD60
	case "MEGAMAN6_GXX":
		return 0x006AACAC
	case "ROCKEXE6_RXX":
		return 0x006CBE80
	case "ROCKEXE6_GXX":
		return 0x006C9DB4
	}
	return 0
}

func tall2MetricsOffset(gameTitle string) int64 {
	switch gameTitle {
	case "MEGAMAN6_FXX":
		return 0x00043CA4
	case "MEGAMAN6_GXX":
		return 0x00043C74
	case "ROCKEXE6_RXX":
		return 0x00044EEC
	case "ROCKEXE6_GXX":
		return 0x00044EBC
	}
	return 0
}

func FindROMInfo(romID string, gameTitle string) *ROMInfo {
	switch romID {
	case "BR6E", "BR6P", "BR5E", "BR5P":
		return &ROMInfo{
			0x0001D854, 0x0001C824, tall2Offset(gameTitle), tall2MetricsOffset(gameTitle),
			[]rune(" 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ*abcdefghijklmnopqrstuvwxyz�����ウアイオエケコカクキセサソシステトツタチネノヌナニヒヘホハフミマメムモヤヨユロルリレラン熱斗ワヲギガゲゴグゾジゼズザデドヅダヂベビボバブピパペプポゥァィォェュヴッョャ-×=:%?+█�ー!&,゜.・;'\"~/()「」�_�����あいけくきこかせそすさしつとてたちねのなぬにへふほはひめむみもまゆよやるらりろれ�んをわ研げぐごがぎぜずじぞざでどづだぢべばびぼぶぽぷぴぺぱぅぁぃぉぇゅょっゃ容量全木�無現実◯✗緑道不止彩起父集院一二三四五六七八陽十百千万脳上下左右手来日目月獣各人入出山口光電気綾科次名前学校省祐室世界高朗枚野悪路闇大小中自分間系花問究門城王兄化葉行街屋水見終新桜先生長今了点井子言太属風会性持時勝赤代年火改計画職体波回外地員正造値合戦川秋原町晴用金郎作数方社攻撃力同武何発少教以白早暮面組後文字本階明才者向犬々ヶ連射舟戸切土炎伊夫鉄国男天老師堀杉士悟森霧麻剛垣"),
		}
	case "BR6J", "BR5J":
		return &ROMInfo{
			0x0001DC78, 0x0001CC48, tall2Offset(gameTitle), tall2MetricsOffset(gameTitle),
			[]rune(" 0123456789ウアイオエケコカクキセサソシステトツタチネノヌナニヒヘホハフミマメムモヤヨユロルリレラン熱斗ワヲギガゲゴグゾジゼズザデドヅダヂベビボバブピパペプポゥァィォェュヴッョャABCDEFGHIJKLMNOPQRSTUVWXYZ*-×=:%?+■�ー!��&、゜.・;’\"~/()「」����_�周えおうあいけくきこかせそすさしつとてたちねのなぬにへふほはひめむみもまゆよやるらりろれ�んをわ研げぐごがぎぜずじぞざでどづだぢべばびぼぶぽぷぴぺぱぅぁぃぉぇゅょっゃabcdefghijklmnopqrstuvwxyz容量全木�無現実◯✗緑道不止彩起父集院一二三四五六七八陽十百千万脳上下左右手来日目月獣各人入出山口光電気綾科次名前学校省祐室世界高朗枚野悪路闇大小中自分間系花問究門城王兄化葉行街屋水見終新桜先生長今了点井子言太属風会性持時勝赤代年火改計画職体波回外地員正造値合戦川秋原町晴用金郎作数方社攻撃力同武何発少教以白早暮面組後文字本階明才者向犬々ヶ連射舟戸切土炎伊夫鉄国男天老師堀杉士悟森霧麻剛垣"),
		}
	}
	return nil
}

var fontPalette = color.Palette{
	color.RGBA{0, 0, 0, 0},
	color.RGBA{0, 0, 0, 255},
}

func ReadGlyph(r io.Reader, opaqueColor uint8) (*image.Alpha, error) {
	glyph := image.NewAlpha(image.Rect(0, 0, 8, 16))
	for o := 0; o < 2; o++ {
		tile, err := sprites.ReadTile(r, image.Rect(0, 0, 8, 8))
		if err != nil {
			return nil, err
		}

		for j := 0; j < 8; j++ {
			for i := 0; i < 8; i++ {
				if tile.Pix[j*8+i] == opaqueColor {
					glyph.Pix[(j+o*8)*8+i] = 0xff
				}
			}
		}
	}

	return glyph, nil
}

func Read16x12Glyph(r io.Reader) (*image.Alpha, error) {
	tile, err := sprites.ReadTile(r, image.Rect(0, 0, 16, 12))
	if err != nil {
		return nil, err
	}

	glyph := image.NewAlpha(tile.Bounds())
	for j := 0; j < tile.Rect.Dy(); j++ {
		for i := 0; i < tile.Rect.Dx(); i++ {
			val := uint8(0)
			switch tile.Pix[j*tile.Rect.Dx()+i] {
			case 1:
				val = 0xff
			case 3:
				val = 0x20
			}
			glyph.Pix[j*tile.Rect.Dx()+i] = val
		}
	}

	return glyph, nil
}
func ReadMetrics(r io.Reader, n int) ([]int, error) {
	widths := make([]int, n)
	for i := 0; i < len(widths); i++ {
		var buf [1]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return nil, err
		}
		widths[i] = int(buf[0])
	}
	return widths, nil
}
