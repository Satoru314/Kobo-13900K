package usecase

// AIModel is one entry of the AI catalog: a provider to run, the model to ask
// for, and the label the GUI shows. IDs are stored in the database, so they must
// stay stable when the catalog is edited.
type AIModel struct {
	ID       string
	Label    string
	Provider string
	Model    string
	// OffPeak is when the model is cheap. New off_peak tasks use the schedule
	// of the development model in the setting.
	OffPeak OffPeakSchedule
}

// Catalog is the AI models kobo may run, read from ai.json at startup.
type Catalog interface {
	Get(id string) (AIModel, bool)
	List() []AIModel
}
