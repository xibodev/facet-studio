package view

import "github.com/xibodev/facet-studio/pkg/view"

// Aliases to pkg/view to preserve backwards compatibility.
type Primitive = view.Primitive

const (
	Video     = view.Video
	Audio     = view.Audio
	Image     = view.Image
	ImageGrid = view.ImageGrid
	Timeline  = view.Timeline
	Markdown  = view.Markdown
	Text      = view.Text
	JSON      = view.JSON
	Document  = view.Document
	Diagram   = view.Diagram
	Slides    = view.Slides
	Table     = view.Table
	Download  = view.Download
)

type Registry = view.Registry
type Card = view.Card
type ViewDefinition = view.ViewDefinition
type SectionDefinition = view.SectionDefinition
type ActionDefinition = view.ActionDefinition

func NewRegistry() *Registry { return view.NewRegistry() }
func FromMediaType(mt string) Primitive { return view.FromMediaType(mt) }
var fromMediaType = view.FromMediaType
var known = view.Known
func All() []Primitive { return view.All() }
func Describe(p Primitive) string { return view.Describe(p) }
