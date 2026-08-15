# Assets

## banner.jpg, banner-no-logo.jpg, and their 2x variants

Repo banner and GitHub social preview, 1280 x 640, with 2560 x 1280 variants beside them. `banner.jpg` carries the mark and is the one embedded in the README. `banner-no-logo.jpg` drops it, for surfaces that already show the avatar alongside, Discord among them.

The mark and the type are vector, so the 2x pair is redrawn rather than enlarged and only the generated field scales. All four are JPEG: the banner is continuous-tone gradient, the content PNG handles worst, and a lossless 1280 PNG came to 558 KB against this repository's 500 KB cap.

Both name `sirens-echo` and `sirens-deep` together, separated by the house ` // `, over the description this repository already gives itself. Naming both is deliberate: the repository houses two deployments and the banner says so.

The previous banner was an original LCARS-idiom composition rendered from a committed HTML file. Both it and that file are retired, and with them the Paramount attribution they required.

See [banner-generation.md](banner-generation.md) for the checkpoint, seed, prompts, type treatment, and the failures the negative prompt encodes.

## sirens-deep.svg, sirens-deep.png

Canonical mark for the `sirens-deep` deployment. `sirens-deep.svg` is the source, `sirens-deep.png` a 512px render for avatar surfaces.

A sibling of the coilyco org marks. The ring and the ink / mint / lilac palette are measured from those avatars rather than approximated, so the geometry matches: mint ring r 165.5 w 12, lilac ring r 153 w 13. The whale carries the emblem alone, with no S, which is what makes this the canonical form rather than the lockup below.

The whale nods at DeepSeek, which `agents/deep/definition.yaml` selects. It is not a trace of their mark. Theirs is a soft organic silhouette; this one is redrawn as a stepped, blocked polygon in the coilyco idiom, and only the gesture is shared - mass low and left, fluke thrown up and right, a single eye dot. Every edge is horizontal or vertical apart from the two running from the fluke down to the belly. The DeepSeek name and logo are property of DeepSeek.

**The water inset is load bearing.** The lilac sea is clipped to a disc of r 132, not to the ring interior at r 146.5. That gap keeps the water off the lilac ring band. Filling to the ring interior makes the two lilacs merge and the lower half of the ring stops reading as a ring. Keep the whale off that boundary too, currently by 22px, or it hugs the water's curved edge.

## sirens-deep-lockup.svg, sirens-deep-lockup.png

Secondary lockup, the same whale over the coilyco S. Matches the org avatars most closely, since they all carry the S, so reach for it where the mark needs to sit in an obvious row with its siblings. The S is one round-capped stroke of w 21 following `M245 182 H165 V229 H240 V283 H158`, with an offset mint bar across its waist.

## Regenerating the mark rasters

The SVGs keep an opaque field so they stand alone, but the PNG renders are transparent outside a circle of r 169.9, just inside the ring's outer edge at r 171.5. Each render is therefore a coin rather than a square tile, and drops onto any surface without dark corners. The same convention, at its own radius, governs the copies of these marks in the website repository.

Render at the target size, clip the alpha to that circle, and zero the fully transparent pixels. Do not bake the field into the PNG.
