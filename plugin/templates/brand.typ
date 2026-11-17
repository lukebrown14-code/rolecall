// Default look for the resume and cover letter ("Header Band" template).
// Imported by both templates as /templates/brand.typ -- build.py stages the
// templates into a temporary --root, so the absolute import resolves there.
//
// ATS safety: the band is a page-background shape; every word on it is real
// text. Body stays single column, no icons, no layout tables.

#let band = rgb("#17363d")        // deep slate -- header band and headings
#let band-accent = rgb("#f2c38f") // light amber, text on the band
#let band-dim = rgb("#d9e2e4")    // contact line on the band
#let marker = rgb("#d17a28")      // the square before each section heading
#let date-colour = rgb("#9a5317") // dates; darker amber for 4.5:1 on white
#let ink = rgb("#1a1f22")
#let dim = rgb("#5d676c")
#let chip = rgb("#eef2f2")
#let note-fill = rgb("#f6f1ea")

#let font-stack = ("Hanken Grotesk", "Helvetica Neue", "Helvetica")

// Top margin is shared by every page; the band fills it on page 1 only.
#let top-margin = 1.3cm
// No longer used by the default band, which grows to fit its contents; kept so
// an older Workspace override that still names it compiles.
#let band-height = 4.1cm

// Kept for templates that set `background: page-background`: the band is now
// drawn by band-header itself, so it can grow with its contents.
#let page-background = none

// Flatten arbitrary content down to a plain string. Pandoc emits heading
// bodies as sequences (smart punctuation splits them), so `.text` is unsafe.
// There is no public `space` variable, so it is detected via [ ].func().
#let space-fn = [ ].func()
#let to-plain(c) = {
  if type(c) == str { c }
  else if type(c) == content {
    let f = c.func()
    if f == space-fn { " " }
    else if f == text { c.text }
    else if c.has("children") { c.children.map(to-plain).join("") }
    else if c.has("body") { to-plain(c.body) }
    else if c.has("text") { c.text }
    else { "" }
  } else { "" }
}

// One contact item as a link where it is an address: mailto for an email, tel
// for a phone number, https for a bare domain or a full URL. Anything else
// (a location) stays plain text. The text shown is always the item as written.
#let contact-item(s) = {
  let s = s.trim()
  if s.match(regex("^[^@\s]+@[^@\s]+\.[^@\s]+$")) != none {
    link("mailto:" + s, s)
  } else if s.match(regex("^https?://\S+$")) != none {
    link(s, s)
  } else if s.match(regex("^(www\.)?[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+(/\S*)?$")) != none {
    link("https://" + s, s)
  } else if s.match(regex("^\+?[0-9][0-9 ()-]{6,}$")) != none {
    link("tel:" + s.replace(regex("[^0-9+]"), ""), s)
  } else { s }
}

// The band: the Owner's name, then the headline with the work-rights tag
// beside it, then the contact row. Everything sits in reading order on its own
// line, because PDF-to-text readers (and so an ATS) read a right-aligned item
// in a side column after the left one, not in place. The band fills the page
// to the top and sides (outset; the page clips it) and grows with its contents.
// `contact` is the " | "-separated string from build.py; it is kept a string
// because `@` in content mode is a label reference. `name` is the Owner's name
// from the Profile (build.py passes {{NAME}}).
#let band-header(headline: "", contact: "", rights: "", name: "") = block(
  width: 100%,
  height: auto,
  breakable: false,
  fill: band,
  outset: (x: 5cm, top: top-margin + 1cm),
  inset: (top: 0.45cm, bottom: 0.5cm),
  {
    show link: set text(fill: band-dim)
    set par(leading: 0.65em, spacing: 0pt)
    block(below: 0.55em, text(size: 24pt, weight: "bold", fill: white, name))
    // The tag's stroke is outset, so the gap before it stays one word gap wide
    // and a PDF reader keeps it on the headline's line.
    block(below: 0.85em, {
      text(size: 11pt, weight: "medium", fill: band-accent, headline)
      if rights != "" {
        h(0.9em)
        box(
          stroke: 0.6pt + band-accent,
          radius: 2pt,
          outset: (x: 4pt, y: 3pt),
          text(size: 8pt, weight: "semibold", tracking: 0.04em, fill: band-accent, rights),
        )
      }
    })
    // The separator is real text, so the extracted row reads "a | b | c". Each
    // item is boxed: a long row wraps between items, never inside a link.
    text(size: 8pt, fill: band-dim,
      contact.split(" | ").filter(s => s.trim() != "").map(s => box(contact-item(s)))
        .join([ #text(fill: band-accent)[|] ]))
  },
)

// Section heading: orange square, then the title in letterspaced capitals.
// A grid, so a long title (the letter's subject line) wraps with a hanging
// indent. Text top-edge is cap height, so dy centres the square on the caps.
// Tracking stays at 0.08em: at 0.1em pdftotext splits the capitals into
// single letters ("P R O F E S S I O N A L"), which an ATS reads as noise.
#let section-heading(body) = block(width: 100%, breakable: false, sticky: true, grid(
  columns: (auto, 1fr),
  column-gutter: 6pt,
  align: top + left,  // explicit: the letter nests this in a bottom-aligned grid
  move(dy: 0.5pt, box(width: 5.5pt, height: 5.5pt, fill: marker)),
  text(size: 9pt, weight: "bold", tracking: 0.08em, fill: band, upper(body)),
))
