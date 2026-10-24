export namespace main {
	
	export class ArtworkGame {
	    id: number;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new ArtworkGame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class ArtworkSettings {
	    configured: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ArtworkSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configured = source["configured"];
	        this.message = source["message"];
	    }
	}
	export class Candidate {
	    score?: number;
	    key: string;
	    name: string;
	    image: string;
	    url: string;
	    provider: string;
	    exact: boolean;
	    aliases: string[];
	    preview?: string;
	    author?: string;
	    width?: number;
	    height?: number;
	
	    static createFrom(source: any = {}) {
	        return new Candidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.score = source["score"];
	        this.key = source["key"];
	        this.name = source["name"];
	        this.image = source["image"];
	        this.url = source["url"];
	        this.provider = source["provider"];
	        this.exact = source["exact"];
	        this.aliases = source["aliases"];
	        this.preview = source["preview"];
	        this.author = source["author"];
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}
	export class BackgroundRecord {
	    provider?: string;
	    image?: string;
	    appid?: string;
	    name?: string;
	    url?: string;
	    source_url?: string;
	    status: string;
	    message: string;
	    checked_at?: string;
	    width?: number;
	    height?: number;
	    candidates?: Candidate[];
	
	    static createFrom(source: any = {}) {
	        return new BackgroundRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.image = source["image"];
	        this.appid = source["appid"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.source_url = source["source_url"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.checked_at = source["checked_at"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.candidates = this.convertValues(source["candidates"], Candidate);
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
	
	export class EnrichmentResult {
	    name: boolean;
	    cover: boolean;
	    background: boolean;
	    review: boolean;
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new EnrichmentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.cover = source["cover"];
	        this.background = source["background"];
	        this.review = source["review"];
	        this.errors = source["errors"];
	    }
	}
	export class MetadataReview {
	    pending: boolean;
	    reason: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new MetadataReview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pending = source["pending"];
	        this.reason = source["reason"];
	        this.count = source["count"];
	    }
	}
	export class Game {
	    id: string;
	    name: string;
	    source: string;
	    executables: string[];
	    appid: any;
	    exe: string;
	    folder: string;
	    image: string;
	    hero_image: string;
	    background: BackgroundRecord;
	    has_cover: boolean;
	    original_name: string;
	    detected_appid: string;
	    favorite: boolean;
	    last_played: string;
	    metadata: Record<string, any>;
	    review: MetadataReview;
	
	    static createFrom(source: any = {}) {
	        return new Game(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.source = source["source"];
	        this.executables = source["executables"];
	        this.appid = source["appid"];
	        this.exe = source["exe"];
	        this.folder = source["folder"];
	        this.image = source["image"];
	        this.hero_image = source["hero_image"];
	        this.background = this.convertValues(source["background"], BackgroundRecord);
	        this.has_cover = source["has_cover"];
	        this.original_name = source["original_name"];
	        this.detected_appid = source["detected_appid"];
	        this.favorite = source["favorite"];
	        this.last_played = source["last_played"];
	        this.metadata = source["metadata"];
	        this.review = this.convertValues(source["review"], MetadataReview);
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
	export class Library {
	    updated: string;
	    games: Game[];
	    note: string;
	    theme: string;
	    data_dir: string;
	
	    static createFrom(source: any = {}) {
	        return new Library(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.updated = source["updated"];
	        this.games = this.convertValues(source["games"], Game);
	        this.note = source["note"];
	        this.theme = source["theme"];
	        this.data_dir = source["data_dir"];
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
	
	export class ScanSettings {
	    roots: string[];
	    excluded: string[];
	
	    static createFrom(source: any = {}) {
	        return new ScanSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.roots = source["roots"];
	        this.excluded = source["excluded"];
	    }
	}

}

