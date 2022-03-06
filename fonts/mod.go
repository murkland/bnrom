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
	Charmap    []rune
}

func FindROMInfo(romID string) *ROMInfo {
	switch romID {
	case "BR6E", "BR6P", "BR5E", "BR5P":
		return &ROMInfo{0x0001D854, 0x0001C824, []rune(" 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ*abcdefghijklmnopqrstuvwxyz�����ウアイオエケコカクキセサソシステトツタチネノヌナニヒヘホハフミマメムモヤヨユロルリレラン熱斗ワヲギガゲゴグゾジゼズザデドヅダヂベビボバブピパペプポゥァィォェュヴッョャ-×=:%?+█�ー!&,゜.・;'\"~/()「」�_�����あいけくきこかせそすさしつとてたちねのなぬにへふほはひめむみもまゆよやるらりろれ�んをわ研げぐごがぎぜずじぞざでどづだぢべばびぼぶぽぷぴぺぱぅぁぃぉぇゅょっゃ容量全木�無現実◯✗緑道不止彩起父集院一二三四五六七八陽十百千万脳上下左右手来日目月獣各人入出山口光電気綾科次名前学校省祐室世界高朗枚野悪路闇大小中自分間系花問究門城王兄化葉行街屋水見終新桜先生長今了点井子言太属風会性持時勝赤代年火改計画職体波回外地員正造値合戦川秋原町晴用金郎作数方社攻撃力同武何発少教以白早暮面組後文字本階明才者向犬々ヶ連射舟戸切土炎伊夫鉄国男天老師堀杉士悟森霧麻剛垣")}
	case "BR6J", "BR5J":
		return &ROMInfo{0x0001DC78, 0x0001CC48, []rune(" 0123456789ウアイオエケコカクキセサソシステトツタチネノヌナニヒヘホハフミマメムモヤヨユロルリレラン熱斗ワヲギガゲゴグゾジゼズザデドヅダヂベビボバブピパペプポゥァィォェュヴッョャABCDEFGHIJKLMNOPQRSTUVWXYZ*-×=:%?+■�ー!��&、゜.・;’\"~/()「」����_�周えおうあいけくきこかせそすさしつとてたちねのなぬにへふほはひめむみもまゆよやるらりろれ�んをわ研げぐごがぎぜずじぞざでどづだぢべばびぼぶぽぷぴぺぱぅぁぃぉぇゅょっゃabcdefghijklmnopqrstuvwxyz容量全木�無現実◯✗緑道不止彩起父集院一二三四五六七八陽十百千万脳上下左右手来日目月獣各人入出山口光電気綾科次名前学校省祐室世界高朗枚野悪路闇大小中自分間系花問究門城王兄化葉行街屋水見終新桜先生長今了点井子言太属風会性持時勝赤代年火改計画職体波回外地員正造値合戦川秋原町晴用金郎作数方社攻撃力同武何発少教以白早暮面組後文字本階明才者向犬々ヶ連射舟戸切土炎伊夫鉄国男天老師堀杉士悟森霧麻剛垣")}
	}
	return nil
}

var fontPalette = color.Palette{
	color.RGBA{0, 0, 0, 0},
	color.RGBA{0, 0, 0, 255},
}

func ReadGlyph(r io.ReadSeeker, opaqueColor uint8) (*image.Paletted, error) {
	glyph := image.NewPaletted(image.Rect(0, 0, 8, 16), fontPalette)
	for o := 0; o < 2; o++ {
		tile, err := sprites.ReadTile(r)
		if err != nil {
			return nil, err
		}

		for j := 0; j < 8; j++ {
			for i := 0; i < 8; i++ {
				if tile.Pix[j*8+i] == opaqueColor {
					glyph.Pix[(j+o*8)*8+i] = 1
				}
			}
		}
	}

	return glyph, nil
}
