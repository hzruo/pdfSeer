/// <reference types="vite/client" />

declare module '*.vue' {
    import type {DefineComponent} from 'vue'
    const component: DefineComponent<{}, {}, any>
    export default component
}

// Wails runtime types
declare global {
    interface Window {
        runtime?: {
            EventsOn: (eventName: string, callback: (data: any) => void) => void;
            EventsOff: (eventName: string) => void;
            EventsEmit: (eventName: string, data?: any) => void;
        };
    }
}
