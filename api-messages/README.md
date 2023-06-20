# Definitions of data structures and messages for comms between UI and API

## How to define messages
- Messages sent through web socket protobuf communications are all wrapped in
  WSMessage, so make sure to add it to the oneof clause in websocket.proto
- The field added to the oneof clause must have the same name as the message name, starting
  in lower case. For example: 
        UserDetailsUpd userDetailsUpd = 1;
  **NOTE: the name starts with lower case**
- Messages where we want to associate a request and response/updates must have the suffix
  Req, Resp and Upd and the name prefix must be the same
- For HTTP request/responses, ensure they don't end in Req, Resp or Upd to not confuse
  the mappings for WSMessage. Define these in restmsgs.proto

## Multiple Files
It's clear this is all too big to fit in one proto file, so it has been broken up into many

As a convention, messages sent via REST calls go into restmsgs.proto while the rest of the
proto files are broken up by themes, like all messages for datasets, all for quants, etc

## Implementation Differences

Route Permissions:
(deprecated) DELETE /annotation/{DID}/{id}                               -> write:data-analysis
(deprecated) GET    /annotation/{DID}/{id}                               -> read:data-analysis
(deprecated) PUT    /annotation/{DID}/{id}                               -> write:data-analysis
(deprecated) GET    /annotation/{DID}                                    -> read:data-analysis
(deprecated) POST   /annotation/{DID}                                    -> write:data-analysis
(deprecated) POST   /share/annotation/{DID}/{id}                         -> write:shared-annotation

(deprecated) POST   /metrics/{id}                                        -> write:metrics

(deprecated, was only for fixing user names...) POST   /user/bulk-user-details                              -> write:user-roles

(re-implement as test msg, see teest-msgs.proto) POST   /notification/test                                   -> write:piquant-config

(This becomes an actual HTTP GET) GET    /dataset/download/{DID}/{id}                         -> public

Tags: is dataset ID required in all msgs?
Datasets: renamed to scans, trying to make this a side-thing, data is requested separately
RGB Mix: renamed to expression group, not limited to "R", "G", "B" but has an array of expressions to run
Users: Broke this up - separate user-hints-msgs.proto, user-nofications-msgs.proto, user-msgs.proto and user-management-msgs.proto
       User config (name, email, data collection) all done via UserDetailsWriteReq
       Managment stuff like roles, listing users, changing user roles is handled in user-management-msgs.proto
ROI: Combined put, post and bulk-upload into a single request
Expressions: Combined put, post into single request
             See ExpressionWriteResultReq for beginnings of storage of expression result
Dataset: Instead of providing a "dataset.bin" file as before, the new API will allow downloading more granularly:
    - Images - all image data can be queried by ImageListReq, and we have ImageUploadReq/ImageDeleteReq
    - Spectra - queried via SpectrumReq
    - Locations - queried via ScanLocationReq
    - Beam Locations - queried via ScanImageLocationsReq
    - Pseudo Intensities - queried via PseudoIntensityReq 
Quantification: Implement "jobs" as agnostic things, which you can ask to run a PIQUANT job in. Jobs need
                run, listing, get-log msgs... maybe delete?
                Still need quant specific msgs for listing/get/upload/export

Sharing needs to be re-thought:
POST   /share/element-set/{id}                              -> write:shared-element-set
POST   /share/rgb-mix/{id}                                  -> write:shared-expression
POST   /share/data-expression/{id}                          -> write:shared-expression
POST   /share/doi/data-expression/{id}                      -> write:shared-expression
POST   /share/quantification/{DID}/{id}                     -> write:shared-quantification
POST   /share/roi/{DID}/{id}                                -> write:shared-roi
POST   /share/view-state-collection/{DID}/{id}              -> write:pixlise-settings
POST   /share/view-state/public/{DID}/{id}                  -> write:pixlise-settings
POST   /share/view-state/{DID}/{id}                         -> write:pixlise-settings

## Proto Written
-GET    /detector-config/{id}                                -> read:piquant-config

-DELETE /diffraction/manual/{DID}/{id}                       -> write:diffraction-peaks
-GET    /diffraction/manual/{DID}                            -> read:diffraction-peaks
-POST   /diffraction/manual/{DID}                            -> write:diffraction-peaks

-DELETE /diffraction/status/{DID}/{id}                       -> write:diffraction-peaks
-GET    /diffraction/status/{DID}                            -> read:diffraction-peaks
-POST   /diffraction/status/{statusid}/{DID}/{id}            -> write:diffraction-peaks

-DELETE /element-set/{id}                                    -> write:data-analysis
-GET    /element-set/{id}                                    -> public
-PUT    /element-set/{id}                                    -> write:data-analysis
-GET    /element-set                                         -> public
-POST   /element-set                                         -> write:data-analysis

-POST   /export/files/{DID}                                  -> export:map

-GET    /logger/fetch/{logStream}                            -> read:logs
-PUT    /logger/level/{logLevel}                             -> write:log-level
-GET    /logger/level                                        -> read:logs

-DELETE /rgb-mix/{id}                                        -> write:data-analysis
-PUT    /rgb-mix/{id}                                        -> write:data-analysis
-GET    /rgb-mix                                             -> public
-POST   /rgb-mix                                             -> write:data-analysis

-GET    /user/all-roles                                      -> read:user-roles
-GET    /user/by-id/{id}                                     -> read:user-roles
-GET    /user/by-role/{id}                                   -> read:user-roles
-GET    /user/query                                          -> read:user-roles
-DELETE /user/roles/{user_id}/{id}                           -> write:user-roles
-POST   /user/roles/{user_id}/{id}                           -> write:user-roles
-GET    /user/roles/{user_id}                                -> read:user-roles

-GET    /user/config                                         -> read:user-settings
-POST   /user/config                                         -> write:user-settings
-PUT    /user/field/{field_id}                               -> write:user-settings

-GET    /piquant/config/{id}/{versions}                      -> read:piquant-config
-GET    /piquant/config/{id}/{version}/{version}             -> read:piquant-config
-GET    /piquant/config                                      -> read:piquant-config
-GET    /piquant/version                                     -> read:piquant-config
-POST   /piquant/version                                     -> write:piquant-config

-PUT    /data-expression/execution-stat/{id}                 -> write:data-analysis
-DELETE /data-expression/{id}                                -> write:data-analysis
-GET    /data-expression/{id}                                -> public
-PUT    /data-expression/{id}                                -> write:data-analysis
-GET    /data-expression                                     -> public
-POST   /data-expression                                     -> write:data-analysis

-GET    /data-module/{id}/{version}                          -> public
-PUT    /data-module/{id}                                    -> write:data-analysis
-GET    /data-module                                         -> public
-POST   /data-module                                         -> write:data-analysis

-GET    /notification/subscriptions                          -> public
-POST   /notification/subscriptions                          -> public

-GET    /notification/hints                                  -> public
-POST   /notification/hints                                  -> public

-GET    /notification/alerts                                 -> public
-POST   /notification/global                                 -> write:piquant-config

-DELETE /dataset/images/{DID}/{imgtype}/{image}              -> write:dataset
-GET    /dataset/images/{DID}/{imgtype}/{image}              -> public
-POST   /dataset/images/{DID}/{imgtype}/{image}              -> write:dataset
-PUT    /dataset/images/{DID}/{imgtype}/{image}              -> write:dataset
-GET    /dataset/images/{DID}/{imgtype}                      -> public
