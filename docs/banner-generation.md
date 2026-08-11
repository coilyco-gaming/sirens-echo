# Banner generation

How the banner field was made, and what to feed the runtime to make it again.

Only the field is generated. The mark and every word are vector and composited afterwards, which is why the prompts below exclude marine life and text rather than asking the model to draw either. Getting a whale or a wordmark out of a diffusion model at this size is a losing game, and there is no reason to play it when both already exist as paths.

## Parameters

ComfyUI writes the full generation graph into every PNG it produces, so a source render carries its own provenance. Compositing and JPEG conversion both discard it, which is why the parameters are recorded here rather than trusted to a file that has already been through two lossy steps.

- **checkpoint** - `Juggernaut-XL_v9_RunDiffusionPhoto_v2.safetensors`
- **size** - 1536 x 768, a true 2:1 close to SDXL's pixel budget, reduced afterwards
- **seed** - 4102
- **steps** - 30, **cfg** - 7.0, **sampler** - `dpmpp_2m`, **scheduler** - `normal`

## Positive prompt

```text
a single broad voluminous sweep of light curving through a near-black field, its lower
edge glowing mint cyan and its upper edge deepening into violet, one large calm curve of
soft luminous mass, palette: near-black ink foundation, mint cyan and deep violet only,
composition: wide 2:1 banner, the left third stays quiet near-black so a mark can sit
there, detail: one continuous gesture and nothing else, generous emptiness, render:
smooth gradient falloff, soft luminous edge, controlled glow, high polish, flat
front-facing screen-space graphical visualization, no physical object, no perspective,
no ground plane, orthographic and diagrammatic
```

## Negative prompt

```text
aircraft, airplane, plane, wing, glider, jet, vehicle, boat, chart, graph, plot, axis,
axes, gridlines, tick marks, data visualization, plastic, acetate, sheet, strip, cloth,
fabric, silk, textile, paper, aurora, northern lights, mountain, terrain, landscape,
hills, sky, clouds, text, letters, words, typography, numbers, labels, logo, watermark,
microtype, whale, fish, marine animal, creature, photographic, perspective, vanishing
point, ground plane, horizon line, three dimensional scene, visual clutter, muddy
contrast, chaotic glitch noise, low quality, white background, gray background, grey
background, magenta background, orange, yellow, red, green
```

The negative prompt is long because it is a record of what actually went wrong, not a precaution. Vehicles are listed because a sweeping band with a gradient produced aircraft, twice. Chart furniture is listed because asking for a waveform produced stock charts with invented tick labels, five times. Aurora, mountain and landscape are listed because a mint band on black is one token away from northern lights, and northern lights arrive with a mountain range underneath them. Removing a term reintroduces its failure.

## Composition notes

Over-specifying the layout cost more than it bought. Asking for the gesture to sit along the bottom, the upper half to stay empty and the left third to stay clear at the same time left the model nowhere to put a shape, and it returned bare fields and a terrain hump. The prompt above asks only for a quiet left third and lets the curve fall where it will, which is what produced the accepted field.

Type is Avenir Next: Demi Bold 50 for the names in lilac with the separator in mint, Regular 31 for the description in mint. It sits on a centred dark halo rather than an offset drop shadow. An offset implies a light direction the mark does not have, and it protects only one side of each glyph, while a centred blur darkens the bed under every edge equally.

See [assets.md](assets.md) for the shipped files.
