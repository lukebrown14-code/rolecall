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

// Top margin is shared by every page; the band covers it on page 1 only.
#let top-margin = 1.3cm
#let band-height = 4.1cm

#let page-background = context {
  if here().page() == 1 {
    place(top + left, rect(width: 100%, height: band-height, fill: band))
  }
}

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

// The band's contents. `contact` is the " | "-separated string from build.py;
// it is kept a string because `@` in content mode is a label reference. `name`
// is the Owner's name from the Profile (build.py passes {{NAME}}).
#let band-header(headline: "", contact: "", rights: "", name: "") = block(
  width: 100%,
  height: band-height - top-margin,
  breakable: false,
  {
    grid(
      columns: (1fr, auto),
      align: (left + bottom, right + bottom),
      stack(
        spacing: 0.5em,
        text(size: 24pt, weight: "bold", fill: white, name),
        text(size: 11pt, weight: "medium", fill: band-accent, headline),
      ),
      box(
        stroke: 0.6pt + band-accent,
        radius: 2pt,
        inset: (x: 6pt, y: 4pt),
        text(size: 8pt, weight: "semibold", tracking: 0.04em, fill: band-accent, rights),
      ),
    )
    place(bottom + left, dy: -0.55cm,
      text(size: 8pt, fill: band-dim,
        contact.split(" | ").map(s => [#s]).join([ #h(0.9em, weak: true) ])))
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
