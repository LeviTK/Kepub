# T1b XML required-subset candidate

This is an implementation checkpoint, not parent acceptance, an independent
code/demo approval, complete T1, or permission to enter T2. It starts at the
parent's unpushed local documentation baseline `72988bf` (not `origin/main`).
The main README, CLI contract, development plan, dependencies and S0 assets
remain parent-owned and unchanged by this product increment.

## Shared processing and scope

- Publication summaries, navigation/references, metadata/content writes and
  history/source reconstruction use `internal/xmltext`. Original UTF-8/UTF-16
  bytes, BOM, declaration and non-target bytes are not reserialized.
- Internal ELEMENT models, ATTLIST types/defaults/#FIXED, general/parameter
  entities (including markup), and NOTATION have bounded grammar processing.
  This is non-validating XML processing: DTD validity constraints are not
  silently converted into well-formedness constraints. First parsed entity,
  attribute and notation declarations have deterministic precedence.
- Character-reference construction, general-reference bypass, attributes and
  balanced entity content are distinct contexts. Attribute defaults normalize
  when declared, not after forward declarations become available. Constructed
  CR is not physically EOL-normalized a second time.
- XML Fifth Edition lexical Names are validated separately from Go's older
  Appendix-B Name table. A byte-length-preserving internal name adapter restores
  original names before namespace, duplicate and structure checks. It does not
  alter source bytes or grant writable locations.
- Effective OPF version and actual manifest MIME govern policy. Appendix B's
  exact external DOCTYPE tuples are not filename guesses, catalogs, downloads
  or exceptions for external ENTITY declarations. NOTATION identifiers are
  declaration data, not read/execute authority. No external XML resolver exists.
- Unread entities retain literal unknown text and original raw-reference spans.
  Unknown namespace/required identity yields capability diagnostics rather than
  invented namespaces or XML malformation. Unread PE suppresses later entity/
  attribute registration under the non-standalone rule.
- Optional `xmlCoverage` reports original hashes, sorted BookPaths, unresolved
  reference/expansion origins and parsed notation identifiers in the frozen
  `data`/`value` locations. It does not widen the request's reading scope or
  implement the later five-dimensional capability schema. XML partial does not
  downgrade an existing business-level block. Content/search reject incomplete
  text; virtual entity text/markup cannot become a direct writable interval.
- Raw/decoded/expanded/work, declaration, entity-depth/replacement and existing
  depth/token/index budgets are separate. Internal-subset comments/PIs are
  markupdecl productions and count toward the 4096 declaration limit.

## Checkpoint evidence and retained failures

Persistent raw logs are kept outside tracked assets. Initial DTD red tests and
old blanket-DTD expectations are retained. The new Fifth-Edition Name probe
first failed Go's Name table and subsequently passed without changing bytes.
The first CLI partial-search probe incorrectly expected error `data` to be nil;
the existing envelope carries a zero-valued result. Its exit-1 log is retained;
the corrected probe checks the real exit-3 diagnostic and absence of residual
matches, without modifying that envelope behavior.

The parent's trailing-Misc report was independently reproduced on this working
tree: NBSP, numeric whitespace references, empty/space entity references before
or after the root produced 24 failures across UTF-8/UTF-16 LE/BE. CDATA outside
the root was already rejected (six positive rejection controls). Lexical S and
reference checks now run **before expansion**; all 30 rejection controls and
legal Misc/inside-root controls pass. The real pinned checker independently
reports RSC-016 for trailing NBSP, CDATA, numeric reference and empty entity.

Latest normal T1b-targeted invocation passed: CLI four tests (39.498 s), XML
thirteen tests plus fifteen UTF-8/UTF-16 entity-source fuzz seeds, navigation and
reference partial/block tests. Earlier complete xmltext/publication/metadata/
references normal suites passed; metadata's T1b-specific integration is exercised
by the CLI default-version metadata edit/history/accept/export loop. Actual
EPUBCheck 5.3.0 validates the defaulted OPF/PE/generated-markup control and both
content/metadata formal edit loops, with every ZIP entry compared byte-for-byte.
HTTP declaration/notation probes observed zero requests.

Full root normal/race/vet, live fuzz and authorized-original regression results
are pending at this checkpoint. Their raw terminal completion/exit evidence,
not this checkpoint's targeted pass, determines their eventual status. Parent
CLI acceptance and fixed-tree independent product audit also remain pending.

## Post-checkpoint corrections

The original `2d62ad0` tree subsequently completed root normal/race/vet and
both live fuzz runs with actual exit 0 and no tracked drift (entity-source:
177005 executions/46.014 s; encoding: 24139 executions/31.016 s). These results
do **not** cover the following product changes.

The parent's navigation counterexample is reproduced and corrected: an unknown
href/src or unread default cannot establish a target, existence, or a missing
link diagnostic. Attribute-specific certainty preserves known links despite
unknown labels or unrelated attributes; known missing targets and structural
blocks remain diagnosed. Twelve EPUB3/EPUB2 asymmetric controls pass, and the
parent's exact synthetic EPUB now returns a null unknown target/existence while
retaining its known sibling and original coverage identity.

The initial blanket generated-text write guard was an overrestriction, not a
contract limitation. Following the parent's explicit ruling, local replacement
now proves literal opening-tag boundaries and the entire literal closing tag
through source spans, independently of decoded body text. Known entity text in
a physical simple-text element can be replaced as one original inner interval;
no-op preserves references, and declarations/other references stay unchanged.
Virtual nodes, unknown content and non-simple content remain unwritable. Twelve
UTF-8/UTF-16 LE/BE controls cover empty-only/empty-ends, nesting, PE declarations
and defaults; the former fuzz assertion and empty-entity budget assertion are
corrected with independent byte expectations and genuine virtual-node controls.
All initial red and intermediate failure logs are retained.

Fixed Namespaces §7 also requires NCNames for entity/notation names and PI
targets, and QNames for DTD element/attribute names. Twelve independent initial
red controls are now rejected; qualified element/attribute names, colons in
identifiers/NMTOKEN values and Fifth-Edition names remain positive controls.

This increment passes complete normal xmltext/publication/metadata/references
suites and all four T1b CLI tests (38.895 s, real pinned checker), including
entity-text content and metadata history/accept/export with exact ZIP bytes.
New-tree root/race/vet/live-fuzz results and independent acceptance remain pending
until their own actual completion evidence is available.
