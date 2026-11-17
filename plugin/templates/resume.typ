// Default resume template -- "Header Band". Colours, fonts and the band live in
// brand.typ, shared with the cover letter.
// Fed by scripts/build.py: pandoc converts resume.md to Typst markup, which is
// injected at the body marker near the foot of this file. build.py also swaps
// the Key Skills paragraph for a skill-chips() call.
//
// ATS safety: single column, real text, no tables for layout, no icons.

#import "/templates/brand.typ": *

#let contact-line = "{{CONTACT}}"
#let contact-line-2 = "{{CONTACT2}}"
#let owner-name = "{{NAME}}"

#set document(title: owner-name + " — Resume", author: owner-name)

#set page(
  paper: "a4",
  margin: (x: 1.6cm, top: top-margin, bottom: 1.6cm),
  background: page-background,
  footer: context {
    // Page numbers only once the resume runs past one page.
    if counter(page).final().first() > 1 {
      set text(size: 7.5pt, weight: "semibold", tracking: 0.1em, fill: dim)
      grid(
        columns: (1fr, auto),
        align: (left, right),
        [#upper(owner-name)],
        [PAGE #counter(page).display() OF #counter(page).final().first()],
      )
    }
  },
)

#set text(font: font-stack, size: 10pt, fill: ink, lang: "en", region: "au")

// No line breaks inside words. A hyphen the template adds, or a line that
// breaks after a compound word's own hyphen, is joined back by PDF-to-text
// readers ("full-platform" becomes "fullplatform"), so an ATS misses the word.
// Boxing a hyphenated word moves the whole word to the next line instead.
#set text(hyphenate: false)
#show regex("\b\w+(?:-\w+)+\b"): box
#set par(justify: false, leading: 0.8em, spacing: 1.1em)

// Heading spacing is set here rather than with v() inside the show rules --
// a leading v() gets absorbed at a block boundary and silently does nothing.
#show heading.where(level: 2): set block(above: 1.7em, below: 0.7em)
#show heading.where(level: 3): set block(above: 1.6em, below: 0.4em)
#show heading.where(level: 4): set block(above: 0.2em, below: 0.55em)

// Level 2, not 1: pandoc maps markdown "## Heading" to Typst "==". The level-1
// slot is the document title, which build.py strips before injection.
#show heading.where(level: 2): it => section-heading(it.body)

// Role lines: "Title — Employer", then the dates on the same line. The dates
// follow the title rather than sit flush right: PDF-to-text readers put a
// right-aligned date in a side column read after the role and its sub-line,
// so an ATS loses which dates belong to which role.
// Written in Markdown as `### Title — Employer | Dates`.
#show heading.where(level: 3): it => {
  let parts = to-plain(it.body).split(" | ")
  block(width: 100%, {
    text(weight: "bold", size: 10.5pt, fill: ink, parts.at(0))
    if parts.len() > 1 {
      h(0.8em)
      text(size: 9pt, weight: "semibold", fill: date-colour, parts.slice(1).join(" | "))
    }
  })
}

// Sub-line under a role (context, e.g. "UK national security consultancy").
#show heading.where(level: 4): it => {
  text(size: 9pt, weight: "regular", fill: dim, it.body)
}

// Key Skills as tags, with a dot between them. PDF-to-text readers read a row
// of words as one line only while each gap stays under about one word gap
// plus a few points; tag padding alone opens gaps wide enough that they read
// the rows as columns, out of order. The dot is real text that bridges each
// gap, so the keywords come out in order on any width of row.
#let skill-chips(items) = {
  set par(leading: 1.05em)
  items.map(s => box(fill: chip, radius: 2pt, inset: (x: 4pt, y: 2.5pt),
    text(size: 9pt, weight: "medium", s))).join(text(size: 9pt, fill: dim)[ · ])
}

#set list(marker: text(fill: dim)[•], indent: 0.3em, body-indent: 0.6em, spacing: 0.7em)
#show list: set par(leading: 0.8em)

#set enum(indent: 0.5em)
#show link: set text(fill: ink)

// ---------------------------------------------------------------- header

#band-header(headline: "{{HEADLINE}}", contact: contact-line, rights: contact-line-2, name: "{{NAME}}")

#v(0.9em)

// ---------------------------------------------------------------- body

{{BODY}}
