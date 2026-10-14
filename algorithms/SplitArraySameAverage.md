- Subtract average from all elements and find subset with sum as 0.
- Also multiple with N, so that no decimal point calculation at all. But I choose to do some interesting double calculation.
- Use bitset<16> bs(num), test(), set(), unset(), to_ullong(), index iteration to operate.
- Double number 3 can be stored in two ways: 2.99999999995 or 3.0000000001, so we can't match, search for doubles. Rather we have to match
  avoiding high precision error like d1-d2 < 1e-12 can be considered equal. Or we can also multiple them with 10^6 and llround() them and then match.
  We can use these keys in map/set etc for matching/searching. Double can't be used directly.

```
class Solution {
public:
    double getSum(int i, vector<double>&numsd, int offset){
        bitset<16>bs(i);
        double ret = 0;
        for(int i=0;i<16;i++)
            if(bs.test(i))
                ret+=numsd[offset+i];
        return ret;
    }
    bool splitArraySameAverage(vector<int>& nums) {
        if(nums.size() == 1)
            return false;
        
        double avg = 0;
        for(auto n: nums)
            avg+=n;
        avg/=nums.size();
        cout << avg << endl;

        vector<double> numsd;
        for(auto n: nums)
            numsd.push_back(-avg+n);
        
        //find a subset with sum == 0        
        int m = nums.size()/2;
        int sizef = m;
        int sizel = nums.size() - m;
        
        int match = -1; //since full array sum is 0 only
        //push all subset sum of first half in set
        unordered_map<long long, int> st;
        for(int i=1;i<(1<<sizef);i++){
            double sum = getSum(i, numsd, 0);
            cout << sum << endl;
            if(abs(sum) <= 1e-12)
                match++;
            st[llround(sum*1000000)]++;
        }

        for(int i=1;i<(1<<sizel);i++){
            double sum = getSum(i, numsd, sizef);
            cout << sum << endl;
            if(abs(sum) <= 1e-12)
                match++;
            match+=st[llround(-1*sum*1000000)];
        }
        return match>0;
    }
};
```
