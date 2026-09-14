***djisktra***

```

class Solution {

public:

int secondMinimum(int n, vector<vector<int>>& edges, int time, int change) {

        //create adjacency list

        vector<vector<int>> adjlist;

adjlist.resize(n+1);

for(int i=0;i<edges.size();i++){

adjlist[edges[i][0]].push_back(edges[i][1]);

adjlist[edges[i][1]].push_back(edges[i][0]);

        }

  


        // djikstra iteration: ordered distance iteration

        priority_queue<pair<int, int>, vector<pair<int,int>>, greater<pair<int,int>>> pq;

pq.push({0,1});

        //two visited array won't do, as we can visit twice same node but with same distance

        // we want two minimum distances.

        vector<int> dist, dist2;

dist.resize(n+1, -1);

dist2.resize(n+1, -1);

while(true){

int curd = [pq.top](http://pq.top)().first;

int curn = [pq.top](http://pq.top)().second;

pq.pop();

if(dist[curn] == -1)

dist[curn] = curd; //first visit

else if(dist[curn] != curd and dist2[curn] == -1)

dist2[curn] = curd; //second min distance visit

else

continue; //subsequent visits. if we add third min distance visit too to intermediate nodes, it will not lead to second minimum distance to n

bool last_node_second_visit = (dist[n]!=-1 and dist2[n]!=-1 and curn == n);

if(last_node_second_visit)

return curd;

  


int sigcycle = (curd/change);

if(sigcycle % 2 == 1) // red cycle

                curd = (sigcycle+1)*change;

  


for(auto& nextn: adjlist[curn]){

pq.push({curd + time, nextn});

            }

  


        }

    }

};



```

