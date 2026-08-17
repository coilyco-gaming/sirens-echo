# Assets

## banner.jpg, banner-no-logo.jpg, and their 2x variants

Repo banner and GitHub social preview, 1280 x 640, with 2560 x 1280 variants beside them. `banner.jpg`
carries the mark and is the one embedded in the README; `banner-no-logo.jpg` drops it, for surfaces that
already show the avatar alongside. **The mark and the type are vector, so the 2x pair is redrawn rather
than enlarged** and only the generated field scales. All four are JPEG: the banner is continuous-tone
gradient, **the content PNG handles worst**, and a lossless 1280 PNG came to 558 KB against this
repository's 500 KB cap. Both name `sirens-echo` and `sirens-deep` together, separated by the house
` // `, **because the repository houses two deployments and the banner says so**. The previous LCARS-idiom
composition and its committed HTML file are retired, and with them the Paramount attribution.

## sirens-deep.svg, sirens-deep.png

Canonical mark for the `sirens-deep` deployment, the SVG being the source and the PNG a 512px avatar
render. **The ring and the ink / mint / lilac palette are measured from the coilyco org avatars rather
than approximated**, so the geometry matches: mint ring r 165.5 w 12, lilac ring r 153 w 13. The whale
carries the emblem alone, with no S, **which is what makes this the canonical form rather than the
lockup**.

The whale nods at DeepSeek, which `agents/deep/definition.yaml` selects. **It is not a trace of their
mark**: theirs is a soft organic silhouette, this one is redrawn as a stepped, blocked polygon in the
coilyco idiom, **and only the gesture is shared** - mass low and left, fluke thrown up and right, a
single eye dot. Every edge is horizontal or vertical apart from the two running from the fluke to the
belly. The DeepSeek name and logo are property of DeepSeek.

**The water inset is load bearing.** The lilac sea is clipped to a disc of r 132, not to the ring
interior at r 146.5, **and that gap keeps the water off the lilac ring band**: filling to the ring
interior makes the two lilacs merge and the lower half of the ring stops reading as a ring. Keep the
whale off that boundary too, currently by 22px, **or it hugs the water's curved edge**.

`sirens-deep-lockup.svg` is the secondary lockup, the same whale over the coilyco S, matching the org
avatars most closely since they all carry the S. The S is one round-capped stroke of w 21 following
`M245 182 H165 V229 H240 V283 H158`, with an offset mint bar across its waist.

**Regenerating the rasters.** The SVGs keep an opaque field so they stand alone, **but the PNG renders
are transparent outside a circle of r 169.9**, just inside the ring's outer edge at r 171.5, so each
render is a coin rather than a square tile **and drops onto any surface without dark corners**. Render
at the target size, clip the alpha to that circle, and zero the fully transparent pixels. **Do not bake
the field into the PNG.**

## Banner generation

**Only the field is generated.** The mark and every word are vector and composited afterwards, **which
is why the prompts exclude marine life and text rather than asking the model to draw either**: getting a
whale or a wordmark out of a diffusion model at this size is a losing game. ComfyUI writes the full
graph into every PNG it produces, **but compositing and JPEG conversion both discard it**, which is why
the parameters are recorded here.

* **checkpoint** `Juggernaut-XL_v9_RunDiffusionPhoto_v2.safetensors`, **size** 1536 x 768 (a true 2:1
  close to SDXL's pixel budget, reduced afterwards), **seed** 4102.
* **steps** 30, **cfg** 7.0, **sampler** `dpmpp_2m`, **scheduler** `normal`.

The positive prompt asks for a single broad voluminous sweep of light curving through a near-black
field, lower edge mint cyan and upper edge deepening into violet, over a near-black ink foundation in
mint and violet only, wide 2:1, **the left third quiet so a mark can sit there**, one continuous gesture
and generous emptiness, smooth gradient falloff and controlled glow, **flat front-facing screen-space
visualization, no physical object, no perspective, no ground plane, orthographic and diagrammatic**.

**The negative prompt is long because it is a record of what actually went wrong, not a precaution.** It
excludes aircraft and vehicles **because a sweeping band with a gradient produced aircraft, twice**;
chart furniture, axes, gridlines, and tick marks **because asking for a waveform produced stock charts
with invented tick labels, five times**; aurora, mountain, terrain, and landscape **because a mint band
on black is one token away from northern lights, and northern lights arrive with a mountain range
underneath them**; plus text, typography, logos, watermarks, marine life, photographic and 3D cues, and
every off-palette colour. **Removing a term reintroduces its failure.**

## Public repository inventory

`list_public_repos` lists an organization's public repositories with description, URL, language, and
last update, answering "what does this org have" **without anyone pasting a list into a prompt, which is
the same staleness problem a written inventory always has**.
`SIRENS_ECHO_REPO_INVENTORY_URL` and `SIRENS_ECHO_REPO_INVENTORY_ORG` switch it on, and **either empty
offers no tool at all**.

**The read is unauthenticated**, with no `Authorization` header set anywhere in `repoinventory.go` and a
test asserting none is sent. **That is the guarantee, rather than a visibility filter**: a filter is a
line of code that can be written wrong, reviewed wrong, or regressed, **while an unauthenticated request
cannot see a private repository at all, so there is nothing for a mistake to leak**. A private or
internal record is still dropped if one arrives, **because a wrong token configured somewhere else must
not turn into a disclosure here**: that check is the belt, having no credential is the braces.

**The inventory is metadata, so it comes from the forge API rather than a mounted tree**, reporting what
the organization has rather than what a mount happens to contain, **and those drift apart the moment a
repository is added**. The listing is capped so one large organization cannot fill a tool result, and
entries sort by name **so two calls a week apart differ only where the org differs**. The dialer is the
fetch tool's, refusing loopback, private ranges, link-local, and CGNAT, **so an inventory pointed at an
internal address fails at connect rather than reaching a cluster service**.

`read_public_file` takes owner, repo, path, and an optional ref, with the same absence of a credential.
**A path that climbs out of the repository is refused before any request is made, and each segment is
escaped separately so a segment carrying a slash cannot forge one.** Output is capped and says so when
it cuts, **because a half file the model cannot tell from a whole one is answered from with ordinary
confidence**. **Neither tool writes anything**, and every URL either returns is one a member could have
opened themselves.

**Why not a mount.** #633 asked for public repositories mounted ward-style so the agent could read its
own source, and these two tools answer that without a volume: **a mounted clone is stale the moment
anything merges, and an API read is current by construction**. A mount still buys what an API read
cannot - grep across a whole tree, following imports, a repository too large to read a file at a time -
and that question stays open.
