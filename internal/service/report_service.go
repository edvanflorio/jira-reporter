package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/alan-gomes1/jira-reporter/internal/config"
	"github.com/alan-gomes1/jira-reporter/internal/model"
	"github.com/alan-gomes1/jira-reporter/internal/repository"
	"github.com/alan-gomes1/jira-reporter/internal/view"
)

// ReportService orquestra a geração de relatórios.
type ReportService interface {
	// Generate gera um relatório com as opções especificadas.
	Generate(opts model.ReportOptions) error
}

// reportService implementa ReportService.
type reportService struct {
	config      *config.Config
	repo        repository.JiraRepository
	dateService DateService
	fileService FileService
	generators  map[model.ReportFormat]view.ReportGenerator
}

// NewReportService cria uma nova instância de ReportService.
func NewReportService(
	cfg *config.Config,
	repo repository.JiraRepository,
	dateService DateService,
	fileService FileService,
	generators map[model.ReportFormat]view.ReportGenerator,
) ReportService {
	return &reportService{
		config:      cfg,
		repo:        repo,
		dateService: dateService,
		fileService: fileService,
		generators:  generators,
	}
}

// Generate gera um relatório com as opções especificadas.
func (s *reportService) Generate(opts model.ReportOptions) error {
	// Validar formato
	if err := s.validateFormat(opts.Format); err != nil {
		return err
	}

	// Buscar dados do Jira
	reportData, err := s.fetchReportData(opts.Date, opts.IncludeQA)
	if err != nil {
		return err
	}

	// Determinar caminhos de arquivo
	paths, err := s.resolvePaths(opts)
	if err != nil {
		return err
	}

	// Gerar o relatório
	if err := s.generateReport(reportData, paths, opts.Format); err != nil {
		return err
	}

	fmt.Printf("Relatório %s gerado com sucesso!\n", paths.finalPath)
	return nil
}

// reportPaths contém os caminhos necessários para geração.
type reportPaths struct {
	directory string
	htmlPath  string
	finalPath string
}

// validateFormat valida se o formato é suportado.
func (s *reportService) validateFormat(format model.ReportFormat) error {
	if !format.IsValid() {
		return fmt.Errorf("formato inválido: %s. Use 'html' ou 'docx'", format)
	}
	if _, exists := s.generators[format]; !exists {
		return fmt.Errorf("gerador não disponível para formato: %s", format)
	}
	return nil
}

// fetchReportData busca e monta os dados do relatório.
func (s *reportService) fetchReportData(
	specifiedDate string, includeQA bool,
) (*model.ReportData, error) {
	var firstDay, lastDay time.Time

	// Usa a data especificada ou o mês anterior como padrão
	if specifiedDate != "" {
		month, year, err := s.dateService.ParseMonthYear(specifiedDate)
		if err != nil {
			return nil, err
		}
		firstDay, lastDay = s.dateService.GetMonthRange(month, year)
	} else {
		firstDay, lastDay = s.dateService.GetPreviousMonthRange()
	}

	issues, err := s.repo.FetchIssues(firstDay, lastDay, includeQA)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar dados do Jira: %w", err)
	}

	user := model.NewUser(
		s.config.CompanyName, s.config.CNPJ, s.config.Username,
	)
	dateWorked := s.dateService.FormatDateWorked(firstDay)

	return model.NewReportData(*user, *issues, dateWorked), nil
}

// resolvePaths determina os caminhos de arquivo para o relatório.
func (s *reportService) resolvePaths(
	opts model.ReportOptions,
) (*reportPaths, error) {
	// Determinar diretório
	directory := opts.Path
	if directory == "" {
		directory = "reports"
	}

	if err := s.fileService.EnsureDir(directory); err != nil {
		return nil, err
	}

	// Gerar nome do arquivo
	fileName := s.generateFileName(opts)
	finalPath := fmt.Sprintf("%s/%s", directory, fileName)

	// Determinar caminho do HTML (pode ser temporário para DOCX)
	htmlPath := finalPath
	if opts.Format == model.FormatDOCX {
		htmlPath = strings.TrimSuffix(finalPath, ".docx") + ".html"
	}

	reportPath := &reportPaths{
		directory: directory,
		htmlPath:  htmlPath,
		finalPath: finalPath,
	}
	return reportPath, nil
}

// generateFileName gera o nome do arquivo baseado nas opções.
func (s *reportService) generateFileName(opts model.ReportOptions) string {
	now := time.Now()
	day := now.Day()
	ext := opts.Format.Extension()

	// Determina o mês/ano para o nome do arquivo
	var monthAndYear string
	if opts.Date != "" {
		monthAndYear = strings.Replace(opts.Date, "/", "_", 1)
	} else {
		// Usa a data atual como padrão (o conteúdo se refere ao mês anterior,
		// mas o nome do arquivo identifica quando o relatório foi emitido).
		monthAndYear = now.Format("01_2006")
	}

	if opts.Name == "" {
		return fmt.Sprintf("report_%d_%s.%s", day, monthAndYear, ext)
	}
	return fmt.Sprintf("%s_%d_%s.%s", opts.Name, day, monthAndYear, ext)
}

// generateReport gera o arquivo de relatório.
func (s *reportService) generateReport(
	data *model.ReportData, paths *reportPaths, format model.ReportFormat,
) error {
	// Sempre gerar HTML primeiro
	htmlGenerator := s.generators[model.FormatHTML]
	htmlFile, err := s.fileService.CreateFile(paths.htmlPath)
	if err != nil {
		return err
	}

	if err := htmlGenerator.Generate(htmlFile, data); err != nil {
		htmlFile.Close()
		return err
	}
	htmlFile.Close()

	// Se for DOCX, converter e remover HTML temporário
	if format == model.FormatDOCX {
		docxGenerator := s.generators[model.FormatDOCX]
		if err := docxGenerator.Generate(
			nil, data, paths.htmlPath, paths.finalPath,
		); err != nil {
			return err
		}
		s.fileService.RemoveFile(paths.htmlPath)
	}

	return nil
}
