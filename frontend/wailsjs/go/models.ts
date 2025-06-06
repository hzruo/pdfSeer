export namespace config {
	
	export class AIConfig {
	    base_url: string;
	    api_key: string;
	    model: string;
	    ocr_model: string;
	    text_model: string;
	    models_endpoint: string;
	    chat_endpoint: string;
	    timeout: number;
	    request_interval: number;
	    burst_limit: number;
	    max_retries: number;
	    retry_delay: number;
	
	    static createFrom(source: any = {}) {
	        return new AIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.base_url = source["base_url"];
	        this.api_key = source["api_key"];
	        this.model = source["model"];
	        this.ocr_model = source["ocr_model"];
	        this.text_model = source["text_model"];
	        this.models_endpoint = source["models_endpoint"];
	        this.chat_endpoint = source["chat_endpoint"];
	        this.timeout = source["timeout"];
	        this.request_interval = source["request_interval"];
	        this.burst_limit = source["burst_limit"];
	        this.max_retries = source["max_retries"];
	        this.retry_delay = source["retry_delay"];
	    }
	}
	export class AppConfig {
	    ai: AIConfig;
	    // Go type: struct { CacheTTL string "json:\"cache_ttl\""; MaxCacheSize string "json:\"max_cache_size\""; HistoryRetention string "json:\"history_retention\"" }
	    storage: any;
	    // Go type: struct { Theme string "json:\"theme\""; DefaultFont string "json:\"default_font\""; Layout string "json:\"layout\"" }
	    ui: any;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ai = this.convertValues(source["ai"], AIConfig);
	        this.storage = this.convertValues(source["storage"], Object);
	        this.ui = this.convertValues(source["ui"], Object);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace document {
	
	export class DocumentInfo {
	    file_path: string;
	    type: string;
	    page_count: number;
	    title: string;
	    author: string;
	    subject: string;
	    supported_ocr: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DocumentInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file_path = source["file_path"];
	        this.type = source["type"];
	        this.page_count = source["page_count"];
	        this.title = source["title"];
	        this.author = source["author"];
	        this.subject = source["subject"];
	        this.supported_ocr = source["supported_ocr"];
	    }
	}

}

export namespace frontend {
	
	export class FileFilter {
	    DisplayName: string;
	    Pattern: string;
	
	    static createFrom(source: any = {}) {
	        return new FileFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.DisplayName = source["DisplayName"];
	        this.Pattern = source["Pattern"];
	    }
	}

}

export namespace history {
	
	export class HistoryPage {
	    id: number;
	    history_id: number;
	    page_number: number;
	    original_text: string;
	    ocr_text: string;
	    ai_processed_text: string;
	    processing_time: number;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.history_id = source["history_id"];
	        this.page_number = source["page_number"];
	        this.original_text = source["original_text"];
	        this.ocr_text = source["ocr_text"];
	        this.ai_processed_text = source["ai_processed_text"];
	        this.processing_time = source["processing_time"];
	        this.created_at = source["created_at"];
	    }
	}
	export class HistoryRecord {
	    id: number;
	    document_path: string;
	    document_name: string;
	    page_count: number;
	    status: string;
	    ai_model: string;
	    cost: number;
	    processed_at: string;
	    completed_at?: string;
	    error_message?: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.document_path = source["document_path"];
	        this.document_name = source["document_name"];
	        this.page_count = source["page_count"];
	        this.status = source["status"];
	        this.ai_model = source["ai_model"];
	        this.cost = source["cost"];
	        this.processed_at = source["processed_at"];
	        this.completed_at = source["completed_at"];
	        this.error_message = source["error_message"];
	    }
	}
	export class SearchResult {
	    history_id: number;
	    document_path: string;
	    document_name: string;
	    page_number: number;
	    snippet: string;
	    processed_at: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.history_id = source["history_id"];
	        this.document_path = source["document_path"];
	        this.document_name = source["document_name"];
	        this.page_number = source["page_number"];
	        this.snippet = source["snippet"];
	        this.processed_at = source["processed_at"];
	    }
	}

}

export namespace ocr {
	
	export class ModelInfo {
	    id: string;
	    name: string;
	    description: string;
	    supports_vision: boolean;
	    max_tokens: number;
	    recommended: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.supports_vision = source["supports_vision"];
	        this.max_tokens = source["max_tokens"];
	        this.recommended = source["recommended"];
	    }
	}

}

export namespace pdf {
	
	export class PDFPage {
	    number: number;
	    text: string;
	    ocr_text: string;
	    ai_text: string;
	    image_path: string;
	    has_text: boolean;
	    width: number;
	    height: number;
	    processed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PDFPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.text = source["text"];
	        this.ocr_text = source["ocr_text"];
	        this.ai_text = source["ai_text"];
	        this.image_path = source["image_path"];
	        this.has_text = source["has_text"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.processed = source["processed"];
	    }
	}
	export class PDFDocument {
	    file_path: string;
	    pages: PDFPage[];
	    page_count: number;
	    title: string;
	    author: string;
	    subject: string;
	
	    static createFrom(source: any = {}) {
	        return new PDFDocument(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file_path = source["file_path"];
	        this.pages = this.convertValues(source["pages"], PDFPage);
	        this.page_count = source["page_count"];
	        this.title = source["title"];
	        this.author = source["author"];
	        this.subject = source["subject"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace system {
	
	export class DependencyStatus {
	    name: string;
	    required: boolean;
	    installed: boolean;
	    version: string;
	    description: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new DependencyStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.required = source["required"];
	        this.installed = source["installed"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.error = source["error"];
	    }
	}
	export class SystemInfo {
	    os: string;
	    arch: string;
	    dependencies: DependencyStatus[];
	
	    static createFrom(source: any = {}) {
	        return new SystemInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.dependencies = this.convertValues(source["dependencies"], DependencyStatus);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

