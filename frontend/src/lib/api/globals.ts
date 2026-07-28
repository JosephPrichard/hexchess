interface State {
    internalBackendBaseURL: string | undefined;
    setInternalBackendBaseURL: (value: string | undefined) => void;
}

const state: State = {
    internalBackendBaseURL: undefined,
    setInternalBackendBaseURL: function(value: string | undefined) {
        this.internalBackendBaseURL = value;
    }
};

export default state;