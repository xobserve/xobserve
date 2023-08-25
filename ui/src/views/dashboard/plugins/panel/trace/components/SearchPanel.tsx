import { Form, FormSection } from "components/form/Form"
import InputSelect from "components/select/InputSelect"
import { useEffect, useMemo, useState } from "react"
import { DatasourceType, Panel } from "types/dashboard"
import { queryJaegerOperations, queryJaegerServices } from "../../../datasource/jaeger/query_runner"
import { isEqual, set, sortBy, uniq } from "lodash"
import { EditorInputItem, EditorNumberItem } from "components/editor/EditorItem"
import { Button, Checkbox, Divider, HStack, Input, Text } from "@chakra-ui/react"
import storage from "utils/localStorage"
import { TraceSearchKey } from "../config/constants"
import { TimeRange } from "types/time"
import { hasVariableFormat, replaceWithVariablesHasMultiValues } from "utils/variable"
import useBus from "use-bus"
import { ShareUrlEvent } from "src/data/bus-events"
import React from "react";
import { useStore } from "@nanostores/react"
import { tracePanelMsg } from "src/i18n/locales/en"
import { shareUrlParams } from "src/views/dashboard/DashboardShare"
import { isEmpty } from "utils/validate"
import { useSearchParams } from "react-router-dom"
import { $variables } from "src/views/variables/store"
import { getDatasource } from "utils/datasource"
interface Props {
    panel: Panel
    onSearch: any
    onSearchIds: any
    dashboardId: string
    timeRange: TimeRange
}

// cache services parsed by variables, when a variable changes, we can compare the new parsed result with cache
// and decide whether need to re-query operations
const traceServicesCache = new Map()
const TraceSearchPanel = ({ timeRange, dashboardId, panel, onSearch, onSearchIds }: Props) => {
    const [searchParams] = useSearchParams()
    const variables = useStore($variables)
    const t1 = useStore(tracePanelMsg)
    const [inited, setInited] = useState(false)
    const [services, setServices] = useState([])
    const [operations, setOperations] = useState([])

    const lastSearch = useMemo(() => storage.get(TraceSearchKey + dashboardId + panel.id) ?? {}, [])
    const [initService, initOperation, initTags, initMax, initMin, initLimit, initTraceIds] = getInitParams(searchParams, panel, lastSearch)
    const [service, setService] = useState<string>(initService)
    const [operation, setOperation] = useState<string>(initOperation)
    const [tags, setTags] = useState<string>(initTags)
    const [max, setMax] = useState<string>(initMax)
    const [min, setMin] = useState<string>(initMin)
    const [limit, setLimit] = useState<number>(initLimit)
    const [traceIds, setTraceIds] = useState<string>(initTraceIds)

    const [useLatestTime, setUseLatestTime] = useState(true)
    const ds = getDatasource(panel.datasource.id)

    useEffect(() => {
        return () => {
            delete traceServicesCache[dashboardId + panel.id]
        }
    }, [])

    useEffect(() => {
        if (inited) {
            if (!isEmpty(traceIds)) {
                onSearchIds(traceIds)
            } else {
                if (service && operation) {
                    onSearch(service, operation, tags, min, max, limit)
                }
            }
        }
    }, [timeRange])

    useEffect(() => {
        loadServices()
    }, [ds.type])

    useEffect(() => {
        if (service) {
            loadOperations()
            onSearch(service, operation, tags, min, max, limit, useLatestTime)
        } else {
            setOperations([])
        }
    }, [service])

    useEffect(() => {
        if (hasVariableFormat(operation)) {
            return
        }

        const services = replaceWithVariablesHasMultiValues(service)
        const cachedServices = traceServicesCache[dashboardId + panel.id]
        if (!isEqual(services, cachedServices)) {
            onVariablesChange()
        }
    }, [variables])

    const onVariablesChange = async () => {
        loadOperations()
        onSearch(service, operation, tags, min, max, limit, useLatestTime)
    }

    useBus(
        ShareUrlEvent,
        () => {
            if (!isEmpty(traceIds)) {
                shareUrlParams['traceIds'] = traceIds
            } else {
                if (!isEmpty(service)) shareUrlParams['service'] = service
                if (!isEmpty(operation)) shareUrlParams['operation'] = operation
                if (!isEmpty(tags)) shareUrlParams['tags'] = tags
                if (!isEmpty(max)) shareUrlParams['max'] = max
                if (!isEmpty(min)) shareUrlParams['min'] = min
                if (!isEmpty(limit)) shareUrlParams['limit'] = limit
            }
        },
        [service, operation, tags, max, min, limit, traceIds]
    )


    const loadServices = async () => {
        switch (ds.type) {
            case DatasourceType.Jaeger:
                const res = await queryJaegerServices(panel.datasource.id)
                const ss = sortBy(res)
                setServices(ss)
                if (ss.length > 0) {
                    if (isEmpty(service)) {
                        setService(ss[0])
                    } 
                }

                setTimeout(() => {
                    setInited(true)
                }, 500)

                break;

            default:
                break;
        }
    }

    const loadOperations = async (s?) => {
        switch (ds.type) {
            case DatasourceType.Jaeger:
                const services = s ?? replaceWithVariablesHasMultiValues(service)
                traceServicesCache[dashboardId + panel.id] = services
                const res = await Promise.all(services.map(service => queryJaegerOperations(panel.datasource.id, service)))

                const ss = sortBy(uniq(res.filter(r => r).flat()))
                setOperations(['all'].concat(ss))

                if (!hasVariableFormat(operation)) {
                    if (!operation || !ss.includes(operation)) {
                        setOperation('all')
                    }
                }

                break;

            default:
                break;
        }
    }

    const onClickSearch = () => {
        if (!isEmpty(traceIds)) {
            onSearchIds(traceIds)
        } else {
            onSearch(service, operation, tags, min, max, limit, useLatestTime)
            storage.set(TraceSearchKey + dashboardId + panel.id, {
                service, operation, tags, min, max, limit
            })
        }
    }

    return (<>
        <Form spacing={4}>
            <FormSection title="Service" titleSize="0.85rem" spacing={1}>
                {
                    panel.plugins.trace.enableEditService ?
                        <InputSelect value={service} options={services.map(s => ({ label: s, value: s }))} size="md" onChange={v => setService(v)} />
                        : <Input value={service} disabled />
                }
            </FormSection>
            <FormSection title="Operation" titleSize="0.85rem" spacing={1}>
                <InputSelect value={operation} options={operations.map(s => ({ label: s, value: s }))} size="md" onChange={v => setOperation(v)} />
            </FormSection>
            <FormSection title="Tags" titleSize="0.85rem" spacing={1}>
                <EditorInputItem value={tags} placeholder={`e.g http.status_code=200 error=true`} onChange={v => setTags(v)} />
            </FormSection>
            <HStack>
                <FormSection title={t1.maxDuration} titleSize="0.85rem" spacing={1}>
                    <EditorInputItem value={max} placeholder="e.g 1.2s,100ms" onChange={v => setMax(v)} />
                </FormSection>
                <FormSection title={t1.minDuration} titleSize="0.85rem" spacing={1}>
                    <EditorInputItem value={min} placeholder="e.g 1.2s,100ms" onChange={v => setMin(v)} />
                </FormSection>
            </HStack>
            <FormSection title={t1.limitResults} titleSize="0.85rem" spacing={1}>
                <EditorNumberItem value={limit} min={0} onChange={v => setLimit(v)} size="md" />
            </FormSection>
            <Divider pt="2" />
            <FormSection title="Trace ids" titleSize="0.85rem" spacing={1} desc={t1.traceIdsTips}>
                <EditorInputItem placeholder={t1.traceIdsInputTips} value={traceIds} onChange={v => setTraceIds(v)} size="md" />
            </FormSection>
            <HStack>
                <Button width="150px" onClick={onClickSearch}>{t1.findTraces}</Button>
                <Checkbox isChecked={useLatestTime} onChange={e => setUseLatestTime(e.currentTarget.checked)} />
                <Text textStyle="annotation">{t1.useLatestTime}</Text>
            </HStack>

        </Form>
    </>)
}

export default TraceSearchPanel


const getInitParams = (searchParams, panel, lastSearch) => {
    let service
    const urlService = searchParams.get('service')
    if (!isEmpty(urlService)) {
        service = urlService
    } else if (!panel.plugins.trace.enableEditService) {
        service = panel.plugins.trace.defaultService
    } else {
        service = lastSearch.service ?? null
    }

    const operation = searchParams.get('operation') ?? (lastSearch.operation ?? null)
    const tags = searchParams.get('tags') ?? (lastSearch.tags ?? '')
    const max = searchParams.get('max') ?? (lastSearch.max ?? '')
    const min = searchParams.get('min') ?? (lastSearch.min ?? '')
    const limit = searchParams.get('limit') ?? (lastSearch.limit ?? 20)
    const traceIds = searchParams.get('traceIds') ?? null
    return [service, operation, tags, max, min, limit, traceIds]
}