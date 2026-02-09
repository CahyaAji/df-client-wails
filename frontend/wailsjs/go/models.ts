export namespace main {
	
	export class Bookmark {
	    id: number;
	    style: string;
	    min_zoom: number;
	    max_zoom: number;
	    north: number;
	    south: number;
	    east: number;
	    west: number;
	    center_lat: number;
	    center_lng: number;
	
	    static createFrom(source: any = {}) {
	        return new Bookmark(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.style = source["style"];
	        this.min_zoom = source["min_zoom"];
	        this.max_zoom = source["max_zoom"];
	        this.north = source["north"];
	        this.south = source["south"];
	        this.east = source["east"];
	        this.west = source["west"];
	        this.center_lat = source["center_lat"];
	        this.center_lng = source["center_lng"];
	    }
	}

}

