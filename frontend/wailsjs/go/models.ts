export namespace controller {
	
	export class Status {
	    connected: boolean;
	    last_err?: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.last_err = source["last_err"];
	    }
	}

}

