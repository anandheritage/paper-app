package domain

// ArXivCategories maps arXiv category IDs to human-readable names.
// Source: https://arxiv.org/category_taxonomy
var ArXivCategories = map[string]CategoryInfo{
	// Computer Science
	"cs.AI": {ID: "cs.AI", Name: "Artificial Intelligence", Group: "Computer Science"},
	"cs.AR": {ID: "cs.AR", Name: "Hardware Architecture", Group: "Computer Science"},
	"cs.CC": {ID: "cs.CC", Name: "Computational Complexity", Group: "Computer Science"},
	"cs.CE": {ID: "cs.CE", Name: "Computational Engineering", Group: "Computer Science"},
	"cs.CG": {ID: "cs.CG", Name: "Computational Geometry", Group: "Computer Science"},
	"cs.CL": {ID: "cs.CL", Name: "Computation and Language", Group: "Computer Science"},
	"cs.CR": {ID: "cs.CR", Name: "Cryptography and Security", Group: "Computer Science"},
	"cs.CV": {ID: "cs.CV", Name: "Computer Vision", Group: "Computer Science"},
	"cs.CY": {ID: "cs.CY", Name: "Computers and Society", Group: "Computer Science"},
	"cs.DB": {ID: "cs.DB", Name: "Databases", Group: "Computer Science"},
	"cs.DC": {ID: "cs.DC", Name: "Distributed Computing", Group: "Computer Science"},
	"cs.DL": {ID: "cs.DL", Name: "Digital Libraries", Group: "Computer Science"},
	"cs.DM": {ID: "cs.DM", Name: "Discrete Mathematics", Group: "Computer Science"},
	"cs.DS": {ID: "cs.DS", Name: "Data Structures and Algorithms", Group: "Computer Science"},
	"cs.ET": {ID: "cs.ET", Name: "Emerging Technologies", Group: "Computer Science"},
	"cs.FL": {ID: "cs.FL", Name: "Formal Languages", Group: "Computer Science"},
	"cs.GL": {ID: "cs.GL", Name: "General Literature", Group: "Computer Science"},
	"cs.GR": {ID: "cs.GR", Name: "Graphics", Group: "Computer Science"},
	"cs.GT": {ID: "cs.GT", Name: "Game Theory", Group: "Computer Science"},
	"cs.HC": {ID: "cs.HC", Name: "Human-Computer Interaction", Group: "Computer Science"},
	"cs.IR": {ID: "cs.IR", Name: "Information Retrieval", Group: "Computer Science"},
	"cs.IT": {ID: "cs.IT", Name: "Information Theory", Group: "Computer Science"},
	"cs.LG": {ID: "cs.LG", Name: "Machine Learning", Group: "Computer Science"},
	"cs.LO": {ID: "cs.LO", Name: "Logic in Computer Science", Group: "Computer Science"},
	"cs.MA": {ID: "cs.MA", Name: "Multiagent Systems", Group: "Computer Science"},
	"cs.MM": {ID: "cs.MM", Name: "Multimedia", Group: "Computer Science"},
	"cs.MS": {ID: "cs.MS", Name: "Mathematical Software", Group: "Computer Science"},
	"cs.NA": {ID: "cs.NA", Name: "Numerical Analysis", Group: "Computer Science"},
	"cs.NE": {ID: "cs.NE", Name: "Neural and Evolutionary Computing", Group: "Computer Science"},
	"cs.NI": {ID: "cs.NI", Name: "Networking", Group: "Computer Science"},
	"cs.OH": {ID: "cs.OH", Name: "Other Computer Science", Group: "Computer Science"},
	"cs.OS": {ID: "cs.OS", Name: "Operating Systems", Group: "Computer Science"},
	"cs.PF": {ID: "cs.PF", Name: "Performance", Group: "Computer Science"},
	"cs.PL": {ID: "cs.PL", Name: "Programming Languages", Group: "Computer Science"},
	"cs.RO": {ID: "cs.RO", Name: "Robotics", Group: "Computer Science"},
	"cs.SC": {ID: "cs.SC", Name: "Symbolic Computation", Group: "Computer Science"},
	"cs.SD": {ID: "cs.SD", Name: "Sound", Group: "Computer Science"},
	"cs.SE": {ID: "cs.SE", Name: "Software Engineering", Group: "Computer Science"},
	"cs.SI": {ID: "cs.SI", Name: "Social and Information Networks", Group: "Computer Science"},
	"cs.SY": {ID: "cs.SY", Name: "Systems and Control", Group: "Computer Science"},

	// Mathematics
	"math.AC": {ID: "math.AC", Name: "Commutative Algebra", Group: "Mathematics"},
	"math.AG": {ID: "math.AG", Name: "Algebraic Geometry", Group: "Mathematics"},
	"math.AP": {ID: "math.AP", Name: "Analysis of PDEs", Group: "Mathematics"},
	"math.AT": {ID: "math.AT", Name: "Algebraic Topology", Group: "Mathematics"},
	"math.CA": {ID: "math.CA", Name: "Classical Analysis", Group: "Mathematics"},
	"math.CO": {ID: "math.CO", Name: "Combinatorics", Group: "Mathematics"},
	"math.CT": {ID: "math.CT", Name: "Category Theory", Group: "Mathematics"},
	"math.CV": {ID: "math.CV", Name: "Complex Variables", Group: "Mathematics"},
	"math.DG": {ID: "math.DG", Name: "Differential Geometry", Group: "Mathematics"},
	"math.DS": {ID: "math.DS", Name: "Dynamical Systems", Group: "Mathematics"},
	"math.FA": {ID: "math.FA", Name: "Functional Analysis", Group: "Mathematics"},
	"math.GM": {ID: "math.GM", Name: "General Mathematics", Group: "Mathematics"},
	"math.GN": {ID: "math.GN", Name: "General Topology", Group: "Mathematics"},
	"math.GR": {ID: "math.GR", Name: "Group Theory", Group: "Mathematics"},
	"math.GT": {ID: "math.GT", Name: "Geometric Topology", Group: "Mathematics"},
	"math.HO": {ID: "math.HO", Name: "History and Overview", Group: "Mathematics"},
	"math.IT": {ID: "math.IT", Name: "Information Theory", Group: "Mathematics"},
	"math.KT": {ID: "math.KT", Name: "K-Theory and Homology", Group: "Mathematics"},
	"math.LO": {ID: "math.LO", Name: "Logic", Group: "Mathematics"},
	"math.MG": {ID: "math.MG", Name: "Metric Geometry", Group: "Mathematics"},
	"math.MP": {ID: "math.MP", Name: "Mathematical Physics", Group: "Mathematics"},
	"math.NA": {ID: "math.NA", Name: "Numerical Analysis", Group: "Mathematics"},
	"math.NT": {ID: "math.NT", Name: "Number Theory", Group: "Mathematics"},
	"math.OA": {ID: "math.OA", Name: "Operator Algebras", Group: "Mathematics"},
	"math.OC": {ID: "math.OC", Name: "Optimization and Control", Group: "Mathematics"},
	"math.PR": {ID: "math.PR", Name: "Probability", Group: "Mathematics"},
	"math.QA": {ID: "math.QA", Name: "Quantum Algebra", Group: "Mathematics"},
	"math.RA": {ID: "math.RA", Name: "Rings and Algebras", Group: "Mathematics"},
	"math.RT": {ID: "math.RT", Name: "Representation Theory", Group: "Mathematics"},
	"math.SG": {ID: "math.SG", Name: "Symplectic Geometry", Group: "Mathematics"},
	"math.SP": {ID: "math.SP", Name: "Spectral Theory", Group: "Mathematics"},
	"math.ST": {ID: "math.ST", Name: "Statistics Theory", Group: "Mathematics"},

	// Physics
	"astro-ph":    {ID: "astro-ph", Name: "Astrophysics", Group: "Physics"},
	"astro-ph.CO": {ID: "astro-ph.CO", Name: "Cosmology and Nongalactic Astrophysics", Group: "Physics"},
	"astro-ph.EP": {ID: "astro-ph.EP", Name: "Earth and Planetary Astrophysics", Group: "Physics"},
	"astro-ph.GA": {ID: "astro-ph.GA", Name: "Astrophysics of Galaxies", Group: "Physics"},
	"astro-ph.HE": {ID: "astro-ph.HE", Name: "High Energy Astrophysical Phenomena", Group: "Physics"},
	"astro-ph.IM": {ID: "astro-ph.IM", Name: "Instrumentation and Methods", Group: "Physics"},
	"astro-ph.SR": {ID: "astro-ph.SR", Name: "Solar and Stellar Astrophysics", Group: "Physics"},
	"cond-mat":    {ID: "cond-mat", Name: "Condensed Matter", Group: "Physics"},
	"gr-qc":       {ID: "gr-qc", Name: "General Relativity and Quantum Cosmology", Group: "Physics"},
	"hep-ex":      {ID: "hep-ex", Name: "High Energy Physics - Experiment", Group: "Physics"},
	"hep-lat":     {ID: "hep-lat", Name: "High Energy Physics - Lattice", Group: "Physics"},
	"hep-ph":      {ID: "hep-ph", Name: "High Energy Physics - Phenomenology", Group: "Physics"},
	"hep-th":      {ID: "hep-th", Name: "High Energy Physics - Theory", Group: "Physics"},
	"math-ph":     {ID: "math-ph", Name: "Mathematical Physics", Group: "Physics"},
	"nlin":        {ID: "nlin", Name: "Nonlinear Sciences", Group: "Physics"},
	"nucl-ex":     {ID: "nucl-ex", Name: "Nuclear Experiment", Group: "Physics"},
	"nucl-th":     {ID: "nucl-th", Name: "Nuclear Theory", Group: "Physics"},
	"physics":     {ID: "physics", Name: "Physics", Group: "Physics"},
	"quant-ph":    {ID: "quant-ph", Name: "Quantum Physics", Group: "Physics"},

	// Quantitative Biology
	"q-bio.BM": {ID: "q-bio.BM", Name: "Biomolecules", Group: "Quantitative Biology"},
	"q-bio.CB": {ID: "q-bio.CB", Name: "Cell Behavior", Group: "Quantitative Biology"},
	"q-bio.GN": {ID: "q-bio.GN", Name: "Genomics", Group: "Quantitative Biology"},
	"q-bio.MN": {ID: "q-bio.MN", Name: "Molecular Networks", Group: "Quantitative Biology"},
	"q-bio.NC": {ID: "q-bio.NC", Name: "Neurons and Cognition", Group: "Quantitative Biology"},
	"q-bio.OT": {ID: "q-bio.OT", Name: "Other Quantitative Biology", Group: "Quantitative Biology"},
	"q-bio.PE": {ID: "q-bio.PE", Name: "Populations and Evolution", Group: "Quantitative Biology"},
	"q-bio.QM": {ID: "q-bio.QM", Name: "Quantitative Methods", Group: "Quantitative Biology"},
	"q-bio.SC": {ID: "q-bio.SC", Name: "Subcellular Processes", Group: "Quantitative Biology"},
	"q-bio.TO": {ID: "q-bio.TO", Name: "Tissues and Organs", Group: "Quantitative Biology"},

	// Quantitative Finance
	"q-fin.CP": {ID: "q-fin.CP", Name: "Computational Finance", Group: "Quantitative Finance"},
	"q-fin.EC": {ID: "q-fin.EC", Name: "Economics", Group: "Quantitative Finance"},
	"q-fin.GN": {ID: "q-fin.GN", Name: "General Finance", Group: "Quantitative Finance"},
	"q-fin.MF": {ID: "q-fin.MF", Name: "Mathematical Finance", Group: "Quantitative Finance"},
	"q-fin.PM": {ID: "q-fin.PM", Name: "Portfolio Management", Group: "Quantitative Finance"},
	"q-fin.PR": {ID: "q-fin.PR", Name: "Pricing of Securities", Group: "Quantitative Finance"},
	"q-fin.RM": {ID: "q-fin.RM", Name: "Risk Management", Group: "Quantitative Finance"},
	"q-fin.ST": {ID: "q-fin.ST", Name: "Statistical Finance", Group: "Quantitative Finance"},
	"q-fin.TR": {ID: "q-fin.TR", Name: "Trading and Market Microstructure", Group: "Quantitative Finance"},

	// Statistics
	"stat.AP": {ID: "stat.AP", Name: "Applications", Group: "Statistics"},
	"stat.CO": {ID: "stat.CO", Name: "Computation", Group: "Statistics"},
	"stat.ME": {ID: "stat.ME", Name: "Methodology", Group: "Statistics"},
	"stat.ML": {ID: "stat.ML", Name: "Machine Learning", Group: "Statistics"},
	"stat.OT": {ID: "stat.OT", Name: "Other Statistics", Group: "Statistics"},
	"stat.TH": {ID: "stat.TH", Name: "Statistics Theory", Group: "Statistics"},

	// Electrical Engineering and Systems Science
	"eess.AS": {ID: "eess.AS", Name: "Audio and Speech Processing", Group: "EESS"},
	"eess.IV": {ID: "eess.IV", Name: "Image and Video Processing", Group: "EESS"},
	"eess.SP": {ID: "eess.SP", Name: "Signal Processing", Group: "EESS"},
	"eess.SY": {ID: "eess.SY", Name: "Systems and Control", Group: "EESS"},

	// Economics
	"econ.EM": {ID: "econ.EM", Name: "Econometrics", Group: "Economics"},
	"econ.GN": {ID: "econ.GN", Name: "General Economics", Group: "Economics"},
	"econ.TH": {ID: "econ.TH", Name: "Theoretical Economics", Group: "Economics"},
}

// ArXivGroups maps top-level group names to their OAI-PMH set identifiers.
var ArXivGroups = map[string]string{
	"Computer Science":      "cs",
	"Mathematics":           "math",
	"Physics":               "physics",
	"Quantitative Biology":  "q-bio",
	"Quantitative Finance":  "q-fin",
	"Statistics":            "stat",
	"EESS":                  "eess",
	"Economics":             "econ",
}

// GetCategoryInfo returns info for a given category ID, with fallback.
func GetCategoryInfo(id string) CategoryInfo {
	if info, ok := ArXivCategories[id]; ok {
		return info
	}
	return CategoryInfo{ID: id, Name: id, Group: "Other"}
}
