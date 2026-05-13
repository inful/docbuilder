package pipeline

const (
	testTitleTest         = "Test"
	testTitleTestSite     = "Test Site"
	testTitleTestPage     = "Test Page"
	testTitleGettingStart = "Getting Started"
	testTitleExisting     = "Existing Title"
	testTitleWelcome      = "Welcome"

	testPermalinkAliasesKey = frontMatterKeyAliases
	testPermalinkAliasUID   = "/_uid/12345/"

	testTitleLineTestPage       = "title: " + testTitleTestPage
	testTitleLineTestPageLF     = testTitleLineTestPage + "\n"
	testTitleLineTestPageCRLF   = testTitleLineTestPage + "\r\n"
	testTitleLineTestPageYAMLLF = "---\n" + testTitleLineTestPageLF
)
