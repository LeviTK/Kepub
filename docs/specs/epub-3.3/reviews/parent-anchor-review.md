# Parent independent review of 12 obsolete official-report anchors

2026-10-06. This is source analysis for the coding Orb's semantic-index integration,
not case execution, renderer verification, or S0 acceptance. Keep original report
URLs, report levels, expectations and source artifacts unchanged. Add explicit
mapping and applicability evidence; do not rewrite history or invent old anchors.

Source snapshots actually read locally:

- `epub`: https://www.w3.org/TR/2026/REC-epub-33-20260113/ ; raw SHA-256 `f927cf3182598c552037445837b73faee346f97d4a396059440830e49e737d99`.
- `rs`: https://www.w3.org/TR/2024/REC-epub-rs-33-20241017/ ; raw SHA-256 `2e8d4400d1cce9080e729df8292d8795b4d76cb1a20be2387a263bcfbcd7ca9e`.
- Also read actual archived XHTML/OPF for `pkg-linked-records`, `fxl-page-spread-center`, `pkg-spine-progression_ltr`, and `lay-fxl-layout-pre-paginated-spreads`.

The paths below use the existing archive DOM parser, but conclusions were made
by reading the full relevant source sections and fixture instructions. These
are proposed links to verify during integration, not a claim that every target
already has a correctly granular matrix row. All execution states stay not-tested.

## Directly corresponding rules

| Case | Old report fragment | Fixed document / anchor or DOM | Actual text and interpretation |
| --- | --- | --- | --- |
| `pub-cmt-mp4` | `cmt-cmt-mp4-aac` | epub `cmt-mp4-aac`; containing row `/html[1]/body[1]/section[5]/section[2]/table[1]/tbody[1]/tr[9]` | `audio/mp4 [mpeg4-audio], [mp4] AAC LC audio using MP4 container`. A repeated `cmt-` typo in the report. Preserve the existing valid rs `confreq-rs-epub3-mp3-aac` reference: audio support is required **if** the reading system can render prerecorded audio. The table row is a media-type definition, not by itself an unconditional playback MUST. |
| `pkg-spine-progression-pre-paginated` | `confreq-rs-spine-progression-prepaginated` | rs `confreq-rs-spine-progression-pre-paginated`, `/html[1]/body[1]/section[7]/section[5]/p[4]` | `If the page-progression-direction attribute has a value other than default, the reading system MUST ignore any directionality computed from pre-paginated XHTML content documents.` The inserted hyphen fixes the identity; retain the condition. |
| `cnt-svg-css-reference` | `confreq-rs-css-embed-ref` | rs `confreq-svg-rs-css-embed-ref`, `/html[1]/body[1]/section[8]/section[1]/section[2]/section[3]/section[1]/p[1]` | `For the purposes of styling SVG embedded in XHTML content documents by reference, reading systems MUST NOT apply CSS style rules of the containing document to the referenced SVG document.` Do not confuse with SVG included inline, which has the opposite CSS application rule. |
| `lay-fxl-layout-pre-paginated-spreads` | `confreq-rs-fxl-prepaginated-spreads` | rs `layout`, `/html[1]/body[1]/section[10]/section[1]/section[1]/section[1]/p[2]` | `When the rendition:layout property is set to pre-paginated, reading systems MUST NOT include space between the adjacent content slots when rendering synthetic spreads.` The fixed paragraph has no individual ID. The fixture describes consecutive pages in spreads; keep its original wording distinct from this rule's conditional no-space requirement. |
| `lay-page-layout-both-spread` | `page-spread-both` | rs `page-spread-flow`, `/html[1]/body[1]/section[10]/section[1]/section[1]/section[4]/p[5]` | `Reading systems MUST honor rendition:page-spread-* properties on both reflowable and pre-paginated spine items (e.g., by inserting a blank page).` This is the applicable two-layout rule, not `page-spread-both-pre-pag`, which addresses the sequence of different layout types. |
| `lay-rendition-flow-pre-pag` | `flow-pre-pag` | rs `flow`, `/html[1]/body[1]/section[10]/section[2]/section[1]/p[5]` | `Reading systems MUST ignore the rendition:flow property and its overrides when processing pre-paginated spine items [epub-33].` The following webtoons note discusses possible future behavior; it does not delete this normative requirement from the fixed REC. |
| `lay-fxl-page-spread-combined` | `fxl-page-spread` | rs `fxl-page-spread-combined`, `/html[1]/body[1]/section[10]/section[1]/section[1]/section[4]/p[7]` | `When a reading system encounters two spine items that represent a true spread (i.e., two adjacent spine items with the rendition:page-spread-left and rendition:page-spread-right properties), it SHOULD create the spread with no space between the adjacent pages.` Keep SHOULD, not MUST. |
| `scr-readingsystem-support_iframe` | `scripting-req-readingsystem-properties` | rs `scripting-req-readingsystem`, `/html[1]/body[1]/section[19]/section[2]/p[2]`; related `confreq-rs-scripted-readingsystem-object` | `Reading systems MUST expose the epubReadingSystem object on the navigator object of all loaded scripted content documents, including any nested container-constrained scripting contexts [epub-33].` The same paragraph also requires availability by DOMContentLoaded. Section 6.4 makes the implementation requirements conditional on scripting support; do not turn CLI script non-execution into a renderer PASS or an unconditional object requirement. |

## Global page progression: two cases need the actual definition

`pkg-spine-progression_ltr` and `pkg-spine-progression_rtl` both refer to missing
rs `confreq-rs-spine-progression`. There is no evidence for inventing that ID on
the new REC. Map to the fixed EPUB definition
`attrdef-spine-page-progression-direction`, under `sec-spine-elem`:

`/html[1]/body[1]/section[7]/section[7]/section[1]/p[5]`

> The page-progression-direction attribute sets the global direction in which the content flows. Allowed values are ltr (left-to-right), rtl (right-to-left) and default. When EPUB creators specify the default value, they are expressing no preference and the reading system can choose the rendering direction.

This is an **unmarked normative definition**, not a new marked MUST. The following
paragraph allows individual content overrides and user mechanisms; preserve those
conditions instead of claiming the attribute forbids every override. The archived
LTR fixture sets the spine value and asks that moving right presents its four
spine documents in order. RS `sec-pkg-doc-spine` / `confreq-rs-spine-progression-default`
provide related reading-order/default processing, not a verbatim replacement for
the obsolete report anchor. Keep the report's `must` classification separate from
the cited definition's source level. Rendering checks belong to S3/S4; parsing and
preservation can be independently mapped to T4 without claiming rendering.

## Center is an alias, not an absent feature

`fxl-page-spread-center` refers to missing rs `page-spread-center`. The fixed EPUB
has that ID on a definition term; its following definition is:

`/html[1]/body[1]/section[10]/section[2]/section[2]/section[4]/dl[1]/dd[1]`

> The rendition:page-spread-center property is an alias of the spread-none property for centering a spine item.

Map this definition **and** rs `spread`, whose `none` definition is:

`/html[1]/body[1]/section[10]/section[1]/section[1]/section[3]/dl[1]/dd[1]/p[1]`

> Reading systems MUST NOT incorporate spine items in a synthetic spread. Reading systems SHOULD create a single viewport positioned at the center of the screen.

The two levels remain distinct. The original fixture is marked deprecated and
uses `rendition:page-spread-center`; do not change the fixture to `spread-none` or
label the alias unsupported solely because the old RS anchor disappeared.

## Linked records require an applicability qualifier

`pkg-linked-records` refers to missing rs `sec-linked-records` and a still-present
EPUB `sec-linked-records`. EPUB `sec-link-elem` explicitly says the specification
does not require reading systems to process linked records. RS `sec-pkg-doc-metadata`
contains the relevant statements:

- `/html[1]/body[1]/section[7]/section[3]/dl[1]/dd[7]/p[1]`: `Retrieval and support of linked resources is OPTIONAL.`
- `title-order`, `/html[1]/body[1]/section[7]/section[3]/dl[1]/dd[3]/p[1]`: the first package `dc:title` MUST be recognized as the main title and presented before other title elements.
- `confreq-rs-pkg-creator-order`, `/html[1]/body[1]/section[7]/section[3]/dl[1]/dd[5]/p[1]`: creator document order determines priority; **if** creator metadata is exposed, all creators SHOULD be included where possible.

The actual archived fixture explicitly says it is relevant only if linked
resources are supported, otherwise the implementation-report result should be
null. It expects package title `Package metadata title!` and creator `Matthew Chan`
despite the linked ONIX having neither. Retain that original conditional test
expectation and link to the separate current metadata rules; do not manufacture
an unconditional creator-display MUST or claim that optional linked records are
always required. Any residual report-to-REC expectation nuance must be disclosed.
