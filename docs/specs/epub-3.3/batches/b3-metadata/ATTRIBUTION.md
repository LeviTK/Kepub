# B3 metadata dependency archive

This batch adds full, unmodified plain-text RFCs from the RFC Editor. It does not
replace the frozen S0 originals, manifest, derived indexes or historical evidence.
The original notices, authors, pagination and copyright restrictions remain in
each file. No excerpts or modified versions of either RFC are distributed here.

- RFC 5646, *Tags for Identifying Languages*, A. Phillips and M. Davis,
  September 2009: copyright 2009 IETF Trust and the identified authors; BCP 78
  and the IETF Trust legal provisions applicable at publication, including the
  stated restrictions on modifications to pre-November-2008 material.
- RFC 5234, *Augmented BNF for Syntax Specifications: ABNF*, D. Crocker and
  P. Overell, January 2008: the original full copyright and intellectual property
  statements are retained at the end of the document.

Sources, actual byte lengths, SHA-256, acquisition date and execution attribution
are in [manifest.json](manifest.json). Acquisition reused the bounded download
function in `scripts/epub33_assets.py`; neither source was reserialized.
This is an asset prerequisite, not a B3 product PASS or independent approval.

B3 uses RFC 5646 §2.1 syntax only, with ABNF notation and ASCII core rules from
RFC 5234. It does not implement registry validation, language negotiation,
preferred-value canonicalization, or RFC 4647 matching. Other References do not
become recursive download or implementation requirements for this syntax-only
increment.
