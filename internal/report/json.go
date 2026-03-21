package report

import (
	"encoding/json"
	"io"

	"github.com/timusus/test-confidence/internal/coverage"
	gitpkg "github.com/timusus/test-confidence/internal/git"
	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/surface"
)

// JSON wrapper types for serialization with proper json tags.
// We use wrapper types rather than modifying model types to keep
// the model package clean and avoid tree-sitter/unexportable fields.

type jsonScanResult struct {
	Path              string                 `json:"path"`
	Platform          string                 `json:"platform"`
	TotalTestFiles    int                    `json:"totalTestFiles"`
	TotalTestMethods  int                    `json:"totalTestMethods"`
	UnparseableFiles  int                    `json:"unparseableFiles,omitempty"`
	AggregateStrength map[string]int         `json:"aggregateStrength,omitempty"`
	FileResults       []jsonFileAnalysis     `json:"fileResults"`
	ScopeDistribution *jsonScopeDistribution `json:"scopeDistribution,omitempty"`
	GitAnalysis       *gitpkg.GitAnalysis    `json:"gitAnalysis,omitempty"`
	SurfaceAnalysis   *jsonSurfaceAnalysis   `json:"surfaceAnalysis,omitempty"`
	RiskHotspots      []jsonRiskHotspot      `json:"riskHotspots,omitempty"`
	ObservedCost      *jsonObservedCost      `json:"observedCost,omitempty"`
	CoverageSummary   *jsonCoverageSummary   `json:"coverageSummary,omitempty"`
	Trends            []jsonTrendSnapshot    `json:"trends,omitempty"`
}

type jsonObservedCost struct {
	TotalUnnecessaryChanges int              `json:"totalUnnecessaryChanges"`
	SetupCoupledCount       int              `json:"setupCoupledCount,omitempty"`
	AnalyzedCommits         int              `json:"analyzedCommits,omitempty"`
	CostlyTests             []jsonCostlyTest `json:"costlyTests,omitempty"`
}

type jsonCostlyTest struct {
	Name               string `json:"name"`
	UnnecessaryChanges int    `json:"unnecessaryChanges"`
	CoChangeRate       int    `json:"coChangeRate"`
}

type jsonTrendSnapshot struct {
	Label            string  `json:"label"`
	TotalTestFiles   int     `json:"totalTestFiles"`
	TotalTestMethods int     `json:"totalTestMethods"`
	BehavioralPct    float64 `json:"behavioralPct"`
	StructuralPct    float64 `json:"structuralPct"`
	VerifyOnly       int     `json:"verifyOnly"`
	ArgumentCaptor   int     `json:"argumentCaptor"`
	StubAndVerify    int     `json:"stubAndVerify"`
	TotalMocks       int     `json:"totalMocks"`
	FakeCount        int     `json:"fakeCount"`
	SurfacesTested   int     `json:"surfacesTested"`
	SurfacesUntested int     `json:"surfacesUntested"`
	SurfacesTotal    int     `json:"surfacesTotal"`
}

type jsonFileAnalysis struct {
	File                    jsonFile                `json:"file"`
	AntiPatterns            []jsonAntiPattern       `json:"antiPatterns,omitempty"`
	Assertions              jsonAssertionAnalysis   `json:"assertions"`
	Structural              []jsonStructural        `json:"structural,omitempty"`
	Tautologies             []jsonTautology         `json:"tautologies,omitempty"`
	Doubles                 jsonDoublesAnalysis     `json:"doubles"`
	Scope                   jsonScopeInfo           `json:"scope"`
	SetupComplexity         jsonSetupComplexity     `json:"setupComplexity"`
	StructuralCouplingScore float64                 `json:"structuralCouplingScore"`
}

type jsonFile struct {
	Path     string `json:"path"`
	Language string `json:"language"`
}

type jsonAntiPattern struct {
	Type     string `json:"type"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Method   string `json:"method,omitempty"`
	Class    string `json:"class,omitempty"`
	Severity int    `json:"severity"`
	Tier     int    `json:"tier"`
}

type jsonAssertionAnalysis struct {
	TotalAssertions        int            `json:"totalAssertions"`
	BehavioralMethods      int            `json:"behavioralMethods"`
	StructuralMethods      int            `json:"structuralMethods"`
	UnclassifiedMethods    int            `json:"unclassifiedMethods"`
	StrengthDistribution   map[string]int `json:"strengthDistribution,omitempty"`
	TargetDistribution     map[string]int `json:"targetDistribution,omitempty"`
	ZeroAssertionMethods   []jsonMethodRef `json:"zeroAssertionMethods,omitempty"`
	OutputInteractionRatio float64        `json:"outputInteractionRatio"`
	UnclassifiedStatements int            `json:"unclassifiedStatements,omitempty"`
}

type jsonMethodRef struct {
	File   string `json:"file"`
	Class  string `json:"class"`
	Method string `json:"method"`
	Line   int    `json:"line"`
}

type jsonStructural struct {
	Type   string `json:"type"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Method string `json:"method"`
	Detail string `json:"detail,omitempty"`
}

type jsonTautology struct {
	File             string `json:"file"`
	Line             int    `json:"line"`
	Method           string `json:"method"`
	StubIdentifier   string `json:"stubIdentifier"`
	AssertIdentifier string `json:"assertIdentifier"`
}

type jsonDoublesAnalysis struct {
	Doubles         []jsonTestDouble    `json:"doubles,omitempty"`
	MockDensity     float64             `json:"mockDensity"`
	SetupOnlyRatio  float64             `json:"setupOnlyRatio"`
	FakeCount       int                 `json:"fakeCount"`
	MostMockedTypes []jsonTypeFrequency `json:"mostMockedTypes,omitempty"`
}

type jsonSetupComplexity struct {
	SetupStatements int     `json:"setupStatements"`
	AssertionCount  int     `json:"assertionCount"`
	Ratio           float64 `json:"ratio"`
}

type jsonTestDouble struct {
	TypeName     string `json:"typeName"`
	VariableName string `json:"variableName"`
	Framework    string `json:"framework"`
	Usage        string `json:"usage"`
	Placement    string `json:"placement"`
}

type jsonTypeFrequency struct {
	TypeName string `json:"typeName"`
	Count    int    `json:"count"`
}

type jsonSurfaceAnalysis struct {
	Surfaces             []jsonSurface           `json:"surfaces"`
	TestedCount          int                     `json:"testedCount"`
	UntestedCount        int                     `json:"untestedCount"`
	UntestedByChurn      []jsonSurfaceChurn      `json:"untestedByChurn,omitempty"`
	UntestedComplexFiles []jsonUntestedComplex   `json:"untestedComplexFiles,omitempty"`
}

type jsonUntestedComplex struct {
	File          string   `json:"file"`
	Name          string   `json:"name"`
	Lines         int      `json:"lines"`
	FunctionCount int      `json:"functionCount"`
	TotalChurn    int      `json:"totalChurn"`
	RecentChurn   int      `json:"recentChurn"`
	LineCoverage  *float64 `json:"lineCoverage,omitempty"`
}

type jsonSurface struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	File           string   `json:"file"`
	Line           int      `json:"line"`
	Lines          int      `json:"lines,omitempty"`
	FunctionCount  int      `json:"functionCount,omitempty"`
	HasTest        bool     `json:"hasTest"`
	TestFile       string   `json:"testFile,omitempty"`
	LineCoverage   *float64 `json:"lineCoverage,omitempty"`
	BranchCoverage *float64 `json:"branchCoverage,omitempty"`
}

type jsonSurfaceChurn struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	File         string   `json:"file"`
	Lines        int      `json:"lines,omitempty"`
	Functions    int      `json:"functionCount,omitempty"`
	GitChurn     int      `json:"gitChurn"`
	RecentChurn  int      `json:"recentChurn,omitempty"`
	LineCoverage *float64 `json:"lineCoverage,omitempty"`
}

type jsonScopeInfo struct {
	Scope         string `json:"scope"`
	IsRobolectric bool   `json:"isRobolectric,omitempty"`
	IsComposeTest bool   `json:"isComposeTest,omitempty"`
	IsRoomTest    bool   `json:"isRoomTest,omitempty"`
}

type jsonScopeDistribution struct {
	Local    int `json:"local"`
	Device   int `json:"device"`
	Snapshot int `json:"snapshot"`
}

type jsonRiskHotspot struct {
	ProductionFile string  `json:"productionFile"`
	TestFile       string  `json:"testFile"`
	Commits        int     `json:"commits"`
	RecentCommits  int     `json:"recentCommits,omitempty"`
	CouplingScore  float64 `json:"couplingScore"`
	VerifyOnly     int     `json:"verifyOnly,omitempty"`
	SetupRatio     float64 `json:"setupRatio,omitempty"`
	ZeroAssertion  int     `json:"zeroAssertion,omitempty"`
	Tautologies    int     `json:"tautologies,omitempty"`
	RiskScore      float64 `json:"riskScore"`
	CostHint       string  `json:"costHint"`
	FixHint        string  `json:"fixHint"`
}

type jsonCoverageSummary struct {
	TotalLinesCovered int     `json:"totalLinesCovered"`
	TotalLinesMissed  int     `json:"totalLinesMissed"`
	OverallLinePct    float64 `json:"overallLinePct"` // 0.0-1.0
	FilesWithData     int     `json:"filesWithData"`
}

// WriteJSONReport writes a JSON representation of the scan result to w.
// gitAnalysis, surfaceAnalysis, riskHotspots, and coverageReport may be nil.
func WriteJSONReport(w io.Writer, result *model.ScanResult, gitAnalysis *gitpkg.GitAnalysis, surfaceAnalysis *surface.SurfaceAnalysis, riskHotspots []RiskHotspot, coverageReport ...*coverage.CoverageReport) error {
	jr := convertScanResult(result)
	sd := result.ScopeDistribution
	if sd.Local > 0 || sd.Device > 0 || sd.Snapshot > 0 {
		jr.ScopeDistribution = &jsonScopeDistribution{
			Local:    sd.Local,
			Device:   sd.Device,
			Snapshot: sd.Snapshot,
		}
	}
	jr.GitAnalysis = gitAnalysis
	if surfaceAnalysis != nil && len(surfaceAnalysis.Surfaces) > 0 {
		jr.SurfaceAnalysis = convertSurfaceAnalysis(surfaceAnalysis)
	}
	for _, rh := range riskHotspots {
		jr.RiskHotspots = append(jr.RiskHotspots, jsonRiskHotspot{
			ProductionFile: rh.ProductionFile,
			TestFile:       rh.TestFile,
			Commits:        rh.Commits,
			RecentCommits:  rh.RecentCommits,
			CouplingScore:  rh.CouplingScore,
			VerifyOnly:     rh.VerifyOnly,
			SetupRatio:     rh.SetupRatio,
			ZeroAssertion:  rh.ZeroAssertion,
			Tautologies:    rh.Tautologies,
			RiskScore:      rh.RiskScore,
			CostHint:       rh.CostHint,
			FixHint:        rh.FixHint,
		})
	}
	// Coverage summary
	if len(coverageReport) > 0 && coverageReport[0] != nil {
		cr := coverageReport[0]
		jr.CoverageSummary = &jsonCoverageSummary{
			TotalLinesCovered: cr.TotalLinesCovered(),
			TotalLinesMissed:  cr.TotalLinesMissed(),
			OverallLinePct:    cr.OverallLineCoverage(),
			FilesWithData:     len(cr.Files),
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jr)
}

func convertSurfaceAnalysis(sa *surface.SurfaceAnalysis) *jsonSurfaceAnalysis {
	jsa := &jsonSurfaceAnalysis{
		TestedCount:   sa.TestedCount,
		UntestedCount: sa.UntestedCount,
	}
	for _, s := range sa.Surfaces {
		js := jsonSurface{
			Name:          s.Name,
			Type:          s.Type.String(),
			File:          s.File,
			Line:          s.Line,
			Lines:         s.Lines,
			FunctionCount: s.FunctionCount,
			HasTest:       s.HasTest,
			TestFile:      s.TestFile,
		}
		if s.HasCoverageData {
			lc := s.LineCoverage
			js.LineCoverage = &lc
			bc := s.BranchCoverage
			js.BranchCoverage = &bc
		}
		jsa.Surfaces = append(jsa.Surfaces, js)
	}
	for _, sc := range sa.UntestedByChurn {
		jsc := jsonSurfaceChurn{
			Name:        sc.Surface.Name,
			Type:        sc.Surface.Type.String(),
			File:        sc.Surface.File,
			Lines:       sc.Surface.Lines,
			Functions:   sc.Surface.FunctionCount,
			GitChurn:    sc.GitChurn,
			RecentChurn: sc.RecentChurn,
		}
		if sc.Surface.HasCoverageData {
			lc := sc.Surface.LineCoverage
			jsc.LineCoverage = &lc
		}
		jsa.UntestedByChurn = append(jsa.UntestedByChurn, jsc)
	}
	for _, ucf := range sa.UntestedComplexFiles {
		juc := jsonUntestedComplex{
			File:          ucf.File,
			Name:          ucf.Name,
			Lines:         ucf.Lines,
			FunctionCount: ucf.FunctionCount,
			TotalChurn:    ucf.TotalChurn,
			RecentChurn:   ucf.RecentChurn,
		}
		if ucf.HasCoverageData {
			lc := ucf.LineCoverage
			juc.LineCoverage = &lc
		}
		jsa.UntestedComplexFiles = append(jsa.UntestedComplexFiles, juc)
	}
	return jsa
}

func convertScanResult(r *model.ScanResult) jsonScanResult {
	jr := jsonScanResult{
		Path:             r.Path,
		Platform:         r.Platform.String(),
		TotalTestFiles:   r.TotalTestFiles,
		TotalTestMethods: r.TotalTestMethods,
		UnparseableFiles: r.UnparseableFiles,
		FileResults:      make([]jsonFileAnalysis, len(r.FileResults)),
	}
	if len(r.AggregateStrength) > 0 {
		jr.AggregateStrength = make(map[string]int)
		for k, v := range r.AggregateStrength {
			jr.AggregateStrength[k.String()] = v
		}
	}
	for i, fa := range r.FileResults {
		jr.FileResults[i] = convertFileAnalysis(fa)
	}
	return jr
}

func convertFileAnalysis(fa model.FileAnalysis) jsonFileAnalysis {
	jfa := jsonFileAnalysis{
		File: jsonFile{
			Path:     fa.File.Path,
			Language: fa.File.Language.String(),
		},
		Assertions:              convertAssertions(fa.Assertions),
		Doubles:                 convertDoubles(fa.Doubles),
		StructuralCouplingScore: fa.StructuralCouplingScore,
	}

	jfa.Scope = jsonScopeInfo{
		Scope:         fa.Scope.Scope.String(),
		IsRobolectric: fa.Scope.IsRobolectric,
		IsComposeTest: fa.Scope.IsComposeTest,
		IsRoomTest:    fa.Scope.IsRoomTest,
	}

	jfa.SetupComplexity = jsonSetupComplexity{
		SetupStatements: fa.SetupComplexity.SetupStatements,
		AssertionCount:  fa.SetupComplexity.AssertionCount,
		Ratio:           fa.SetupComplexity.Ratio,
	}

	for _, ap := range fa.AntiPatterns {
		jfa.AntiPatterns = append(jfa.AntiPatterns, jsonAntiPattern{
			Type:     ap.Type.String(),
			File:     ap.File,
			Line:     ap.Line,
			Method:   ap.Method,
			Class:    ap.Class,
			Severity: int(ap.Severity),
			Tier:     1, // always 1 per spec
		})
	}

	for _, si := range fa.Structural {
		jfa.Structural = append(jfa.Structural, jsonStructural{
			Type:   si.Type.String(),
			File:   si.File,
			Line:   si.Line,
			Method: si.Method,
			Detail: si.Detail,
		})
	}

	for _, t := range fa.Tautologies {
		jfa.Tautologies = append(jfa.Tautologies, jsonTautology{
			File:             t.File,
			Line:             t.Line,
			Method:           t.Method,
			StubIdentifier:   t.StubIdentifier,
			AssertIdentifier: t.AssertIdentifier,
		})
	}

	return jfa
}

func convertAssertions(a model.AssertionAnalysis) jsonAssertionAnalysis {
	ja := jsonAssertionAnalysis{
		TotalAssertions:        a.TotalAssertions,
		BehavioralMethods:      a.BehavioralMethods,
		StructuralMethods:      a.StructuralMethods,
		UnclassifiedMethods:    a.UnclassifiedMethods,
		OutputInteractionRatio: a.OutputInteractionRatio,
		UnclassifiedStatements: a.UnclassifiedStatements,
	}

	if len(a.StrengthDistribution) > 0 {
		ja.StrengthDistribution = make(map[string]int)
		for k, v := range a.StrengthDistribution {
			ja.StrengthDistribution[k.String()] = v
		}
	}

	if len(a.TargetDistribution) > 0 {
		ja.TargetDistribution = make(map[string]int)
		for k, v := range a.TargetDistribution {
			ja.TargetDistribution[k.String()] = v
		}
	}

	for _, m := range a.ZeroAssertionMethods {
		ja.ZeroAssertionMethods = append(ja.ZeroAssertionMethods, jsonMethodRef{
			File:   m.File,
			Class:  m.Class,
			Method: m.Method,
			Line:   m.Line,
		})
	}

	return ja
}

func convertDoubles(d model.DoublesAnalysis) jsonDoublesAnalysis {
	jd := jsonDoublesAnalysis{
		MockDensity:    d.MockDensity,
		SetupOnlyRatio: d.SetupOnlyRatio,
		FakeCount:      d.FakeCount,
	}

	for _, td := range d.Doubles {
		jd.Doubles = append(jd.Doubles, jsonTestDouble{
			TypeName:     td.TypeName,
			VariableName: td.VariableName,
			Framework:    td.Framework.String(),
			Usage:        td.Usage.String(),
			Placement:    td.Placement.String(),
		})
	}

	for _, tf := range d.MostMockedTypes {
		jd.MostMockedTypes = append(jd.MostMockedTypes, jsonTypeFrequency{
			TypeName: tf.TypeName,
			Count:    tf.Count,
		})
	}

	return jd
}

// HTMLReportInput bundles all data needed for the HTML report.
type HTMLReportInput struct {
	Result          *model.ScanResult
	GitAnalysis     *gitpkg.GitAnalysis
	SurfaceAnalysis *surface.SurfaceAnalysis
	RiskHotspots    []RiskHotspot
	History         []Snapshot
	CostData        *HTMLCostData
	Coverage        *coverage.CoverageReport
}

// MarshalJSONReport produces the JSON bytes that drive the HTML viewer.
// It includes all scan data plus optional trends and observed cost.
func MarshalJSONReport(input HTMLReportInput) ([]byte, error) {
	jr := convertScanResult(input.Result)
	sd := input.Result.ScopeDistribution
	if sd.Local > 0 || sd.Device > 0 || sd.Snapshot > 0 {
		jr.ScopeDistribution = &jsonScopeDistribution{
			Local:    sd.Local,
			Device:   sd.Device,
			Snapshot: sd.Snapshot,
		}
	}
	jr.GitAnalysis = input.GitAnalysis
	if input.SurfaceAnalysis != nil && len(input.SurfaceAnalysis.Surfaces) > 0 {
		jr.SurfaceAnalysis = convertSurfaceAnalysis(input.SurfaceAnalysis)
	}
	for _, rh := range input.RiskHotspots {
		jr.RiskHotspots = append(jr.RiskHotspots, jsonRiskHotspot{
			ProductionFile: rh.ProductionFile,
			TestFile:       rh.TestFile,
			Commits:        rh.Commits,
			RecentCommits:  rh.RecentCommits,
			CouplingScore:  rh.CouplingScore,
			VerifyOnly:     rh.VerifyOnly,
			SetupRatio:     rh.SetupRatio,
			ZeroAssertion:  rh.ZeroAssertion,
			Tautologies:    rh.Tautologies,
			RiskScore:      rh.RiskScore,
			CostHint:       rh.CostHint,
			FixHint:        rh.FixHint,
		})
	}

	// Observed cost
	if input.CostData != nil && input.CostData.TotalUnnecessaryChanges > 0 {
		oc := &jsonObservedCost{
			TotalUnnecessaryChanges: input.CostData.TotalUnnecessaryChanges,
			SetupCoupledCount:       input.CostData.SetupCoupledCount,
			AnalyzedCommits:         input.CostData.AnalyzedCommits,
		}
		for _, ct := range input.CostData.CostlyTests {
			oc.CostlyTests = append(oc.CostlyTests, jsonCostlyTest{
				Name:               ct.Name,
				UnnecessaryChanges: ct.UnnecessaryChanges,
				CoChangeRate:       ct.CoChangeRate,
			})
		}
		jr.ObservedCost = oc
	}

	// Coverage summary
	if input.Coverage != nil {
		jr.CoverageSummary = &jsonCoverageSummary{
			TotalLinesCovered: input.Coverage.TotalLinesCovered(),
			TotalLinesMissed:  input.Coverage.TotalLinesMissed(),
			OverallLinePct:    input.Coverage.OverallLineCoverage(),
			FilesWithData:     len(input.Coverage.Files),
		}
	}

	// Trends
	for _, s := range input.History {
		jr.Trends = append(jr.Trends, jsonTrendSnapshot{
			Label:            s.Label,
			TotalTestFiles:   s.TotalTestFiles,
			TotalTestMethods: s.TotalTestMethods,
			BehavioralPct:    s.BehavioralPct,
			StructuralPct:    s.StructuralPct,
			VerifyOnly:       s.VerifyOnly,
			ArgumentCaptor:   s.ArgumentCaptor,
			StubAndVerify:    s.StubAndVerify,
			TotalMocks:       s.TotalMocks,
			FakeCount:        s.FakeCount,
			SurfacesTested:   s.SurfacesTested,
			SurfacesUntested: s.SurfacesUntested,
			SurfacesTotal:    s.SurfacesTotal,
		})
	}

	return json.Marshal(jr)
}
