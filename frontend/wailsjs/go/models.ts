export namespace config {
	
	export class ActionDef {
	    Type: string;
	    Template: string;
	    Command: string;
	    Terminal: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ActionDef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Type = source["Type"];
	        this.Template = source["Template"];
	        this.Command = source["Command"];
	        this.Terminal = source["Terminal"];
	    }
	}
	export class ActionConfig {
	    Name: string;
	    Match: Record<string, string>;
	    Action: ActionDef;
	    Icon: string;
	
	    static createFrom(source: any = {}) {
	        return new ActionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Match = source["Match"];
	        this.Action = this.convertValues(source["Action"], ActionDef);
	        this.Icon = source["Icon"];
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
	
	export class DisplayConfig {
	    VisibleLabels: string[];
	    VisibleAnnotations: string[];
	    GroupBy: string[];
	    SortBy: string;
	
	    static createFrom(source: any = {}) {
	        return new DisplayConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.VisibleLabels = source["VisibleLabels"];
	        this.VisibleAnnotations = source["VisibleAnnotations"];
	        this.GroupBy = source["GroupBy"];
	        this.SortBy = source["SortBy"];
	    }
	}
	export class UIConfig {
	    Theme: string;
	    PopupWidth: number;
	    PopupHeight: number;
	    ShowResolved: boolean;
	    ShowSilenced: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Theme = source["Theme"];
	        this.PopupWidth = source["PopupWidth"];
	        this.PopupHeight = source["PopupHeight"];
	        this.ShowResolved = source["ShowResolved"];
	        this.ShowSilenced = source["ShowSilenced"];
	    }
	}

}

export namespace model {
	
	export class Alert {
	    id: string;
	    source: string;
	    sourceType: string;
	    name: string;
	    severity: string;
	    state: string;
	    labels: Record<string, string>;
	    annotations: Record<string, string>;
	    // Go type: time
	    startsAt: any;
	    // Go type: time
	    updatedAt: any;
	    generatorURL: string;
	    silencedBy: string[];
	    inhibitedBy: string[];
	    receivers: string[];
	
	    static createFrom(source: any = {}) {
	        return new Alert(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source = source["source"];
	        this.sourceType = source["sourceType"];
	        this.name = source["name"];
	        this.severity = source["severity"];
	        this.state = source["state"];
	        this.labels = source["labels"];
	        this.annotations = source["annotations"];
	        this.startsAt = this.convertValues(source["startsAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.generatorURL = source["generatorURL"];
	        this.silencedBy = source["silencedBy"];
	        this.inhibitedBy = source["inhibitedBy"];
	        this.receivers = source["receivers"];
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
	export class SeverityCounts {
	    critical: number;
	    warning: number;
	    info: number;
	
	    static createFrom(source: any = {}) {
	        return new SeverityCounts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.critical = source["critical"];
	        this.warning = source["warning"];
	        this.info = source["info"];
	    }
	}

}

