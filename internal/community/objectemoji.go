package community

import "unicode"

// The neutral profile admits an emoji that names a thing and refuses one that
// carries tone. See docs/sirens-echo-object-emoji.md for where the line sits.

// emotiveRunes are faces, people, body, hearts, and celebration marks. A party
// popper is an object to Unicode and tone to a reader, so the reader wins.
var emotiveRunes = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x263A, Hi: 0x263B, Stride: 1}, // ☺ ☻
		{Lo: 0x2728, Hi: 0x2728, Stride: 1}, // ✨
		{Lo: 0x2764, Hi: 0x2764, Stride: 1}, // ❤
	},
	R32: []unicode.Range32{
		{Lo: 0x1F31F, Hi: 0x1F31F, Stride: 1}, // 🌟
		{Lo: 0x1F386, Hi: 0x1F38A, Stride: 1}, // fireworks through confetti
		{Lo: 0x1F440, Hi: 0x1F450, Stride: 1}, // eyes, nose, mouth, hands
		{Lo: 0x1F464, Hi: 0x1F487, Stride: 1}, // people and body
		{Lo: 0x1F493, Hi: 0x1F49F, Stride: 1}, // hearts
		{Lo: 0x1F4A5, Hi: 0x1F4A5, Stride: 1}, // 💥
		{Lo: 0x1F4AB, Hi: 0x1F4AB, Stride: 1}, // 💫
		{Lo: 0x1F4AF, Hi: 0x1F4AF, Stride: 1}, // 💯
		{Lo: 0x1F590, Hi: 0x1F596, Stride: 1}, // raised hands
		{Lo: 0x1F5A4, Hi: 0x1F5A4, Stride: 1}, // 🖤
		{Lo: 0x1F600, Hi: 0x1F64F, Stride: 1}, // Emoticons: faces and gestures
		{Lo: 0x1F90C, Hi: 0x1F93E, Stride: 1}, // hands, hearts, people
		{Lo: 0x1F970, Hi: 0x1F97A, Stride: 1}, // more faces
		{Lo: 0x1F9B4, Hi: 0x1F9BB, Stride: 1}, // body parts
		{Lo: 0x1F9D0, Hi: 0x1F9DF, Stride: 1}, // people and fantasy people
		{Lo: 0x1FAC0, Hi: 0x1FAC5, Stride: 1}, // organs and people
		{Lo: 0x1FAE0, Hi: 0x1FAFF, Stride: 1}, // newer faces and hands
	},
}

// indicatorRunes are status dots, geometric shapes, and verdict marks. They
// are legible and they are not objects, which is the line Kai drew on #203.
var indicatorRunes = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x2190, Hi: 0x21FF, Stride: 1}, // arrows
		{Lo: 0x25A0, Hi: 0x25FF, Stride: 1}, // geometric shapes
		{Lo: 0x2705, Hi: 0x2705, Stride: 1}, // ✅
		{Lo: 0x274C, Hi: 0x274E, Stride: 1}, // ❌ ❎
		{Lo: 0x2B00, Hi: 0x2BFF, Stride: 1}, // more shapes and arrows
	},
	R32: []unicode.Range32{
		{Lo: 0x1F534, Hi: 0x1F53D, Stride: 1}, // coloured circles and triangles
		{Lo: 0x1F780, Hi: 0x1F7FF, Stride: 1}, // Geometric Shapes Extended: 🟢 🔴
	},
}

// emojiContinuation joins runes into one glyph: the zero-width joiner, the
// emoji presentation selector, and the skin-tone modifiers.
func emojiContinuation(r rune) bool {
	return r == '‍' || r == '️' || (r >= 0x1F3FB && r <= 0x1F3FF)
}

// objectEmojiCount reports how many separate object emoji a reply carries, and
// whether any refused rune appears. A joined sequence counts once.
func objectEmojiCount(reply string) (count int, refused bool) {
	inCluster := false
	for _, current := range reply {
		// ASCII is never decorative, and Sk holds the grave accent, so
		// scanning it would ban a code span. See sirens-echo#203.
		if current < 0x80 {
			inCluster = false
			continue
		}
		if emojiContinuation(current) {
			continue
		}
		if unicode.Is(emotiveRunes, current) || unicode.Is(indicatorRunes, current) {
			return count, true
		}
		if unicode.Is(unicode.So, current) || unicode.Is(unicode.Sk, current) {
			if !inCluster {
				count++
			}
			inCluster = true
			continue
		}
		inCluster = false
	}
	return count, false
}
