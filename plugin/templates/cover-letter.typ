// Default cover letter template -- "Header Band", shared with the resume
// through brand.typ so the pair reads as one set of documents.
// Fed by scripts/build.py: pandoc converts cover-letter.md to Typst markup,
// injected at the body marker below. build.py lifts the letter's first
// paragraph (the "Company — Role (Ref)" line) out into the subject marker.
// The voice rules live in plugin/styles/au.md.

#import "/templates/brand.typ": *

#let contact-line = "{{CONTACT}}"
#let contact-line-2 = "{{CONTACT2}}"
#let owner-name = "{{NAME}}"

#set document(title: owner-name + " — Cover Letter", author: owner-name)

#set page(
  paper: "a4",
  margin: (x: 1.6cm, top: top-margin, bottom: 1.6cm),
  background: page-background,
)
#set text(font: font-stack, size: 10.2pt, fill: ink, lang: "en", region: "au")
#set par(justify: false, leading: 0.85em, spacing: 1.3em, first-line-indent: 0pt)

#show link: set text(fill: ink)
#show heading: it => { v(0.4em); text(weight: "bold", size: 10.2pt, fill: band, it.body); v(0.1em) }

// ---------------------------------------------------------------- header

#band-header(headline: "{{HEADLINE}}", contact: contact-line, rights: contact-line-2, name: "{{NAME}}")

#v(1.1em)

#block(width: 100%, stroke: (bottom: 0.6pt + rgb("#dfe4e5")), inset: (bottom: 0.75em), grid(
  columns: (1fr, auto),
  align: (left + bottom, right + bottom),
  column-gutter: 1em,
  section-heading[{{SUBJECT}}],
  text(size: 9pt, weight: "semibold", fill: date-colour)[{{DATELINE}}],
))

#v(0.6em)

// ---------------------------------------------------------------- body

// The practical note (always opens "One practical note") sits in a tinted box --
// it is the one paragraph the reader must not skim past.
#show par: it => {
  if to-plain(it.body).starts-with("One practical note") {
    block(width: 100%, fill: note-fill, radius: 2pt, inset: (x: 9pt, y: 7pt), it)
  } else { it }
}
#show "{{NAME}}": set text(size: 11.5pt, weight: "bold", fill: band)

{{BODY}}
