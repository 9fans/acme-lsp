package protocol

//metdata is implemented due to the https://github.com/OmniSharp/omnisharp-roslyn/issues/2238
const (
	MetadataEndpoint = "o#/metadata" //omnisharp-roslyn endpoint for fetching metadata from csharp LSP
	// MetadataEndpoint = "csharp/metadata" //csharp-ls endpoint for fetching metadata from csharp LSP
)

// https://github.com/OmniSharp/omnisharp-vscode/blob/7689b0d3dd615224ac890dd9bccf9fb0bdae4a64/src/omnisharp/protocol.ts#L96-L111
//	export interface MetadataSource {
//	    AssemblyName: string;
//	    ProjectName: string;
//	    VersionNumber: string;
//	    Language: string;
//	    TypeName: string;
//	}
//
//	export interface MetadataRequest extends MetadataSource {
//	    Timeout?: number;
//	}
//
//	export interface MetadataResponse {
//	    SourceName: string;
//	    Source: string;
//	}

type MetadataParams OmnisharpRoslynMetadataParams
type MetaSourceRsponse OmnisharpRoslynMetaSourceReponse

type OmnisharpRoslynMetadataParams struct {
	TimeOut int `json:"timeout,omitempty"`
	//
	AssemblyName  string `json:"assemblyName,omitempty"`
	ProjectName   string `json:"projectName,omitempty"`
	VersionNumber string `json:"versionNumber,omitempty"`
	Language      string `json:"language,omitempty"`
	TypeName      string `json:"typeName,omitempty"`
}

type OmnisharpRoslynMetaSourceReponse struct {
	Source     string `json:"source,omitempty"`
	SourceName string `json:"sourceName,omitempty"`
}

//	https://github.com/razzmatazz/csharp-language-server/blob/549740a22cba3217bb431ea06d9e17275c079062/src/CSharpLanguageServer/Server.fs#L71-L80
//	type CSharpMetadataParams = {
//	    TextDocument: TextDocumentIdentifier
//	}
//
//	type CSharpMetadataResponse = {
//	    ProjectName: string;
//	    AssemblyName: string;
//	    SymbolName: string;
//	    Source: string;
//	}

type CsharpLsMetaResponse struct {
	Source       string `json:"source,omitempty"`
	AssemblyName string `json:"assemblyName,omitempty"`
	ProjectName  string `json:"projectName,omitempty"`
	TypeName     string `json:"typeName,omitempty"`
}

type CsharpLsMetadataParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}
